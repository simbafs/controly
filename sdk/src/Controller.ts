/**
 * @file Implements the Controller client for the Controly SDK.
 */

import { ControlyBase } from './ControlyBase.js'
import {
	OutgoingMessage,
	CommandPayload,
	ControllerEventMap,
	StatusPayload,
	CommandListPayload,
	NotificationPayload,
	DisplayDisconnectedPayload,
	ControlyOptions,
} from './types.js'

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
 * controller.on('display_disconnected', (displayId) => {
 *  console.log(`Display ${displayId} has disconnected.`);
 *  // Remove the UI for the disconnected display
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
export class Controller extends ControlyBase<ControllerEventMap> {
	private waitingList: string[] = []

	/**
	 * Creates an instance of a Controller client.
	 * @param options The configuration options for the Controller.
	 */
	constructor(options: ControlyOptions) {
		super(options, {
			type: 'controller',
			id: options.id || '',
		})
	}

	/**
	 * Subscribes to one or more Displays to receive their command lists and status updates.
	 * If a display is offline, it will be added to the waiting list.
	 * @param displayIds An array of Display IDs to subscribe to.
	 * @throws {Error} if the WebSocket is not connected.
	 */
	public subscribe(displayIds: string[]): void {
		this.sendMessage({
			type: 'subscribe',
			payload: { display_ids: displayIds },
		})
	}

	/**
	 * Unsubscribes from one or more Displays. This will also remove them from the waiting list.
	 * @param displayIds An array of Display IDs to unsubscribe from.
	 * @throws {Error} if the WebSocket is not connected.
	 */
	public unsubscribe(displayIds: string[]): void {
		this.sendMessage({
			type: 'unsubscribe',
			payload: { display_ids: displayIds },
		})
	}

	/**
	 * Manually sets the waiting list on the server.
	 * This will overwrite the existing waiting list for this controller.
	 * The server will filter this list, keeping only the IDs of displays that are currently offline.
	 * The controller will receive a `waiting` event with the final, updated list.
	 * @param displayIds An array of Display IDs to wait for.
	 * @throws {Error} if the WebSocket is not connected.
	 */
	public setWaitingList(displayIds: string[]): void {
		this.sendMessage({
			type: 'waiting',
			payload: displayIds,
		})
	}

	/**
	 * Sends a command to a specific Display.
	 * @param displayId The ID of the target Display.
	 * @param command The command object to send.
	 * @throws {Error} if the WebSocket is not connected.
	 */
	public sendCommand(displayId: string, command: CommandPayload): void {
		this.sendMessage({
			type: 'command',
			to: displayId,
			payload: command,
		})
	}

	/**
	 * Returns the current list of display IDs that the controller is waiting for.
	 * @returns {string[]} An array of display IDs.
	 */
	public getWaitingList(): string[] {
		return [...this.waitingList]
	}

	/**
	 * Processes incoming messages from the server, specific to the Controller client.
	 * @param message The parsed message from the server.
	 * @internal
	 */
	protected processMessage(message: OutgoingMessage<any, any>): void {
		const { type, payload, from } = message

		switch (type) {
			case 'status':
				this.emitter.emit('status', payload as StatusPayload, from)
				break
			case 'command_list':
				this.emitter.emit('command_list', payload as CommandListPayload, from)
				break
			case 'notification':
				this.emitter.emit('notification', payload as NotificationPayload, from)
				break
			case 'display_disconnected':
				this.emitter.emit('display_disconnected', (payload as DisplayDisconnectedPayload).display_id)
				break
			case 'waiting':
				this.waitingList = payload || []
				this.emitter.emit('waiting', this.getWaitingList())
				break
			default:
				// Other message types are ignored by the controller.
				break
		}
	}
}
