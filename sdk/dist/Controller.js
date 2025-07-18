/**
 * @file Implements the Controller client for the Controly SDK.
 */
import { ControlyBase } from './ControlyBase';
/**
 * The Controller client for Controly.
 * This class is used for clients that need to control one or more Displays.
 * It connects to the server, subscribes to displays, sends commands, and listens for status updates.
 *
 * @example
 * ```javascript
 * const controller = new controly.Controller({
 *   serverUrl: 'ws://localhost:8080/ws',
 *   id: 'my-remote-controller-A',
 * });
 *
 * controller.on('open', (id) => {
 *   console.log(`Controller connected with ID: ${id}`);
 *   controller.subscribe(['display-01']);
 * });
 *
 * controller.on('command_list', (commandList, fromDisplayId) => {
 *   console.log(`Commands from ${fromDisplayId}:`, commandList);
 *   // Dynamically build a UI based on the commands
 * });
 *
 * controller.on('status', (status, fromDisplayId) => {
 *   console.log(`Status from ${fromDisplayId}:`, status);
 *   // Update the UI with the new status
 * });
 *
 * controller.connect();
 *
 * // Later, to send a command:
 * controller.sendCommand('display-01', {
 *   name: 'set_volume',
 *   args: { level: 90 },
 * });
 * ```
 */
export class Controller extends ControlyBase {
    /**
     * Creates an instance of a Controller client.
     * @param options The configuration options for the Controller.
     */
    constructor(options) {
        super(options.serverUrl, {
            type: 'controller',
            id: options.id || '',
        });
    }
    /**
     * Subscribes to one or more Displays to receive their command lists and status updates.
     * @param displayIds An array of Display IDs to subscribe to.
     * @throws {Error} if the WebSocket is not connected.
     */
    subscribe(displayIds) {
        this.sendMessage({
            type: 'subscribe',
            payload: { display_ids: displayIds },
        });
    }
    /**
     * Unsubscribes from one or more Displays.
     * @param displayIds An array of Display IDs to unsubscribe from.
     * @throws {Error} if the WebSocket is not connected.
     */
    unsubscribe(displayIds) {
        this.sendMessage({
            type: 'unsubscribe',
            payload: { display_ids: displayIds },
        });
    }
    /**
     * Sends a command to a specific Display.
     * @param displayId The ID of the target Display.
     * @param command The command object to send.
     * @throws {Error} if the WebSocket is not connected.
     */
    sendCommand(displayId, command) {
        this.sendMessage({
            type: 'command',
            to: displayId,
            payload: command,
        });
    }
    /**
     * Processes incoming messages from the server, specific to the Controller client.
     * @param message The parsed message from the server.
     * @internal
     */
    processMessage(message) {
        const { type, payload, from } = message;
        switch (type) {
            case 'status':
                this.emitter.emit('status', payload, from);
                break;
            case 'command_list':
                this.emitter.emit('command_list', payload, from);
                break;
            case 'notification':
                this.emitter.emit('notification', payload, from);
                break;
            default:
                // Other message types are ignored by the controller.
                break;
        }
    }
}
