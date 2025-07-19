/**
 * @file Implements the Display client for the Controly SDK.
 */
import { ControlyBase } from './ControlyBase';
/**
 * The Display client for Controly.
 * This class is used for devices that need to be controlled remotely.
 * It connects to the server, registers its commands, and listens for incoming command messages.
 *
 * @example
 * ```javascript
 * const display = new controly.Display({
 *   serverUrl: 'ws://localhost:8080/ws',
 *   id: 'my-unique-display-01',
 *   commandUrl: 'https://example.com/commands.json',
 *   token: 'your-secret-token', // Optional
 * });
 *
 * display.command('play_pause', (args, fromControllerId) => {
 *   console.log(`Received 'play_pause' from ${fromControllerId}`);
 *   // ... implement logic
 *   display.updateStatus({ playback: 'playing' });
 * });
 *
 * display.on('open', (id) => {
 *   console.log(`Display connected with ID: ${id}`);
 * });
 *
 * display.connect();
 * ```
 */
export class Display extends ControlyBase {
    /**
     * Creates an instance of a Display client.
     * @param options The configuration options for the Display.
     */
    constructor(options) {
        super(options.serverUrl, {
            type: 'display',
            id: options.id || '',
            command_url: options.commandUrl,
            token: options.token || '',
        });
        this.commandHandlers = new Map();
        this._subscriberCount = 0;
    }
    /**
     * Registers a handler function for a specific command.
     * When a Controller sends a command with a matching name, this handler is executed.
     *
     * @template T - The expected type of the arguments for this command.
     * @param commandName The name of the command to handle (e.g., 'play_pause').
     * @param callback The function to execute when the command is received.
     * It receives the command arguments and the ID of the originating Controller.
     */
    command(commandName, callback) {
        this.commandHandlers.set(commandName, callback);
    }
    /**
     * Sends a status update to all subscribed Controllers.
     * This should be called whenever the state of the Display changes.
     *
     * @param payload An object representing the current status of the Display.
     * This can be any object that is serializable to JSON.
     * @throws {Error} if the WebSocket is not connected.
     */
    updateStatus(payload) {
        this.sendMessage({
            type: 'status',
            payload,
        });
    }
    /**
     * Returns the current number of controllers subscribed to this Display.
     * @returns The number of subscribed controllers.
     */
    subscribers() {
        return this._subscriberCount;
    }
    /**
     * Processes incoming messages from the server, specific to the Display client.
     * @param message The parsed message from the server.
     * @internal
     */
    processMessage(message) {
        if (message.type === 'command' && message.from) {
            const command = message.payload;
            const handler = this.commandHandlers.get(command.name);
            if (handler) {
                try {
                    handler(command.args || {}, message.from);
                }
                catch (error) {
                    console.error(`Error executing handler for command "${command.name}":`, error);
                }
            }
            else {
                // It's not mandatory to handle all commands, so we just log a warning.
                console.warn(`Received unhandled command: "${command.name}"`);
            }
        }
        else if (message.type === 'subscribed') {
            const payload = message.payload;
            this._subscriberCount = payload.count;
            this.emitter.emit('subscribed', payload, message.from);
        }
        else if (message.type === 'unsubscribed') {
            const payload = message.payload;
            this._subscriberCount = payload.count;
            this.emitter.emit('unsubscribed', payload, message.from);
        }
    }
}
