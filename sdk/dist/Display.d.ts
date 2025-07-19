/**
 * @file Implements the Display client for the Controly SDK.
 */
import { ControlyBase } from './ControlyBase';
import { OutgoingMessage, StatusPayload, CommandHandler, DisplayEventMap } from './types';
/**
 * Represents the options for creating a Display instance.
 */
export interface DisplayOptions {
    /** The WebSocket URL of the relay server. */
    serverUrl: string;
    /** The URL of the `command.json` file for this Display. */
    commandUrl: string;
    /** An optional, self-specified ID for this Display. */
    id?: string;
    /** An optional token for authentication. */
    token?: string;
}
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
export declare class Display extends ControlyBase<DisplayEventMap> {
    private commandHandlers;
    private _subscriberCount;
    /**
     * Creates an instance of a Display client.
     * @param options The configuration options for the Display.
     */
    constructor(options: DisplayOptions);
    /**
     * Registers a handler function for a specific command.
     * When a Controller sends a command with a matching name, this handler is executed.
     *
     * @template T - The expected type of the arguments for this command.
     * @param commandName The name of the command to handle (e.g., 'play_pause').
     * @param callback The function to execute when the command is received.
     * It receives the command arguments and the ID of the originating Controller.
     */
    command<T extends Record<string, any> = Record<string, any>>(commandName: string, callback: CommandHandler<T>): void;
    /**
     * Sends a status update to all subscribed Controllers.
     * This should be called whenever the state of the Display changes.
     *
     * @param payload An object representing the current status of the Display.
     * This can be any object that is serializable to JSON.
     * @throws {Error} if the WebSocket is not connected.
     */
    updateStatus(payload: StatusPayload): void;
    /**
     * Returns the current number of controllers subscribed to this Display.
     * @returns The number of subscribed controllers.
     */
    subscribers(): number;
    /**
     * Processes incoming messages from the server, specific to the Display client.
     * @param message The parsed message from the server.
     * @internal
     */
    protected processMessage(message: OutgoingMessage<any, any>): void;
}
