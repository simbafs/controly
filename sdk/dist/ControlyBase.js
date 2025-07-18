"use strict";
/**
 * @file Implements the base client with common WebSocket logic for Controly.
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.ControlyBase = void 0;
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
class ControlyBase {
    /**
     * Creates an instance of ControlyBase.
     * @param serverUrl The WebSocket URL of the relay server (e.g., 'ws://localhost:8080/ws').
     * @param params URL query parameters to be added to the server URL.
     */
    constructor(serverUrl, params) {
        this.ws = null;
        this.emitter = new EventEmitter();
        this.clientId = null;
        this.handleOpen = () => {
            // The 'open' event is fired after the server assigns an ID via 'set_id' message.
            console.log('WebSocket connection established. Waiting for client ID.');
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
                code: 5000,
                message: 'A WebSocket communication error occurred.',
            };
            this.emitter.emit('error', errorPayload, undefined);
        };
        this.handleClose = () => {
            this.emitter.emit('close');
        };
        const url = new URL(serverUrl);
        Object.entries(params).forEach(([key, value]) => {
            if (value) {
                url.searchParams.set(key, value);
            }
        });
        this.fullUrl = url.toString();
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
            throw new Error('Connection is already active or connecting.');
        }
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
}
exports.ControlyBase = ControlyBase;
