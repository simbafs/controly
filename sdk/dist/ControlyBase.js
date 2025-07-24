/**
 * @file Implements the base client with common WebSocket logic for Controly.
 */
/**
 * An internal, simple event emitter.
 * @template T - A map of event names to their handler types.
 */
class EventEmitter {
    constructor() {
        this.listeners = new Map();
    }
    /**
     * Registers an event listener.
     * @param eventName The name of the event.
     * @param callback The callback function.
     */
    on(eventName, callback) {
        if (!this.listeners.has(eventName)) {
            this.listeners.set(eventName, []);
        }
        this.listeners.get(eventName).push(callback);
    }
    /**
     * Emits an event, calling all registered listeners.
     * @param eventName The name of the event.
     * @param args The arguments to pass to the listeners.
     */
    emit(eventName, ...args) {
        const eventListeners = this.listeners.get(eventName);
        if (eventListeners) {
            eventListeners.forEach(callback => callback(...args));
        }
    }
}
/**
 * Abstract base class for Controly clients, handling common WebSocket functionality.
 * @template EventMap - A map of event names to their handler types.
 */
export class ControlyBase {
    /**
     * Creates an instance of ControlyBase.
     * @param options The connection options.
     * @param params URL query parameters to be added to the server URL.
     */
    constructor(options, params) {
        this.ws = null;
        this.emitter = new EventEmitter();
        this.clientId = null;
        this.reconnectAttempts = 0;
        this.explicitDisconnect = false;
        this.handleOpen = () => {
            this.reconnectAttempts = 0;
            this._log('WebSocket connection established. Waiting for client ID.');
        };
        this.handleMessage = (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type === 'set_id') {
                    const { id } = message.payload;
                    this.clientId = id;
                    this.emitter.emit('open', this.clientId);
                    return;
                }
                if (message.type === 'error') {
                    this.emitter.emit('error', message.payload, message.from);
                    return;
                }
                this.processMessage(message);
            }
            catch (error) {
                console.error('Failed to parse server message:', event.data, error);
                const syntheticError = {
                    code: -1,
                    message: 'Failed to parse server message',
                };
                this.emitter.emit('error', syntheticError, undefined);
            }
        };
        this.handleError = (event) => {
            console.error('WebSocket error:', event);
            const errorPayload = {
                code: 'WEBSOCKET_ERROR',
                message: 'A WebSocket communication error occurred.',
            };
            this.emitter.emit('error', errorPayload, undefined);
        };
        this.handleClose = (event) => {
            this.emitter.emit('close', event);
            if (this.explicitDisconnect || !this.reconnect) {
                return;
            }
            if (this.reconnectAttempts < this.maxRetries) {
                this.reconnectAttempts++;
                this._log(`Connection lost. Attempting to reconnect in ${this.reconnectDelay / 1000}s... (${this.reconnectAttempts}/${this.maxRetries})`);
                this.cleanup();
                setTimeout(() => {
                    this.connect();
                }, this.reconnectDelay);
            }
            else {
                console.error(`Failed to reconnect after ${this.maxRetries} attempts.`);
                this.emitter.emit('error', {
                    code: 'RECONNECT_FAILED',
                    message: `Failed to reconnect after ${this.maxRetries} attempts.`,
                });
            }
        };
        const url = new URL(options.serverUrl);
        Object.entries(params).forEach(([key, value]) => {
            if (value) {
                url.searchParams.set(key, value);
            }
        });
        this.fullUrl = url.toString();
        this.reconnect = options.reconnect ?? true;
        this.maxRetries = options.maxRetries ?? 5;
        this.reconnectDelay = options.reconnectDelay ?? 10 * 1000;
        this.silent = options.silent ?? false;
    }
    /**
     * Registers an event listener for a specific event.
     * @param eventName The name of the event to listen for.
     * @param callback The function to call when the event is emitted.
     */
    on(eventName, callback) {
        this.emitter.on(eventName, callback);
    }
    /**
     * Establishes a connection to the Controly server.
     * @throws {Error} if the connection is already open or in the process of connecting.
     */
    connect() {
        if (this.ws && this.ws.readyState !== WebSocket.CLOSED) {
            this._warn('Connection is already active or connecting.');
            return;
        }
        this.cleanup();
        this.explicitDisconnect = false;
        // Do not reset reconnectAttempts here, allow handleClose to manage it.
        this.ws = new WebSocket(this.fullUrl);
        this.ws.addEventListener('open', this.handleOpen);
        this.ws.addEventListener('message', this.handleMessage);
        this.ws.addEventListener('error', this.handleError);
        this.ws.addEventListener('close', this.handleClose);
    }
    /**
     * Disconnects from the Controly server.
     */
    disconnect() {
        this.explicitDisconnect = true;
        this.cleanup();
    }
    /**
     * Cleans up the WebSocket connection and its event listeners.
     * @private
     */
    cleanup() {
        if (this.ws) {
            this.ws.removeEventListener('open', this.handleOpen);
            this.ws.removeEventListener('message', this.handleMessage);
            this.ws.removeEventListener('error', this.handleError);
            this.ws.removeEventListener('close', this.handleClose);
            if (this.ws.readyState === WebSocket.OPEN) {
                this.ws.close();
            }
            this.ws = null;
        }
    }
    /**
     * Sends a message to the server.
     * @param message The message object to send.
     * @throws {Error} if the WebSocket is not connected.
     */
    sendMessage(message) {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            throw new Error('WebSocket is not connected. Cannot send message.');
        }
        this.ws.send(JSON.stringify(message));
    }
    /**
     * Gets the current client ID.
     * @returns The client ID, or null if not yet assigned.
     */
    getId() {
        return this.clientId;
    }
    /**
     * Logs a message to the console if not in silent mode.
     * @param {...any[]} args - The arguments to log.
     */
    _log(...args) {
        if (!this.silent) {
            console.log(...args);
        }
    }
    /**
     * Logs a warning to the console if not in silent mode.
     * @param {...any[]} args - The arguments to log.
     */
    _warn(...args) {
        if (!this.silent) {
            console.warn(...args);
        }
    }
}
