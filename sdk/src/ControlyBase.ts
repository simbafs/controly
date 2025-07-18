/**
 * @file Implements the base client with common WebSocket logic for Controly.
 */

import { IncomingMessage, OutgoingMessage, MessageType, SetIdPayload, ErrorPayload } from './types'

/**
 * An internal, simple event emitter.
 * @template T - A map of event names to their handler types.
 */
class EventEmitter<T extends Record<string, (...args: any[]) => void>> {
	private listeners: Map<keyof T, Function[]> = new Map()

	/**
	 * Registers an event listener.
	 * @param eventName The name of the event.
	 * @param callback The callback function.
	 */
	public on<E extends keyof T>(eventName: E, callback: T[E]): void {
		if (!this.listeners.has(eventName)) {
			this.listeners.set(eventName, [])
		}
		this.listeners.get(eventName)!.push(callback)
	}

	/**
	 * Emits an event, calling all registered listeners.
	 * @param eventName The name of the event.
	 * @param args The arguments to pass to the listeners.
	 */
	public emit<E extends keyof T>(eventName: E, ...args: any[]): void {
		const eventListeners = this.listeners.get(eventName)
		if (eventListeners) {
			eventListeners.forEach(callback => callback(...args))
		}
	}
}

/**
 * Abstract base class for Controly clients, handling common WebSocket functionality.
 * @template EventMap - A map of event names to their handler types.
 */
export abstract class ControlyBase<EventMap extends Record<string, (...args: any[]) => void>> {
	protected ws: WebSocket | null = null
	protected emitter: EventEmitter<EventMap> = new EventEmitter<EventMap>()
	protected clientId: string | null = null

	/**
	 * The full WebSocket server URL.
	 */
	public readonly fullUrl: string

	/**
	 * Creates an instance of ControlyBase.
	 * @param serverUrl The WebSocket URL of the relay server (e.g., 'ws://localhost:8080/ws').
	 * @param params URL query parameters to be added to the server URL.
	 */
	constructor(serverUrl: string, params: Record<string, string>) {
		const url = new URL(serverUrl)
		Object.entries(params).forEach(([key, value]) => {
			if (value) {
				url.searchParams.set(key, value)
			}
		})
		this.fullUrl = url.toString()
	}

	/**
	 * Registers an event listener for a specific event.
	 * @param eventName The name of the event to listen for.
	 * @param callback The function to call when the event is emitted.
	 */
	public on<E extends keyof EventMap>(eventName: E, callback: EventMap[E]): void {
		this.emitter.on(eventName, callback)
	}

	/**
	 * Establishes a connection to the Controly server.
	 * @throws {Error} if the connection is already open or in the process of connecting.
	 */
	public connect(): void {
		if (this.ws && this.ws.readyState !== WebSocket.CLOSED) {
			throw new Error('Connection is already active or connecting.')
		}

		this.ws = new WebSocket(this.fullUrl)
		this.ws.addEventListener('open', this.handleOpen)
		this.ws.addEventListener('message', this.handleMessage)
		this.ws.addEventListener('error', this.handleError)
		this.ws.addEventListener('close', this.handleClose)
	}

	/**
	 * Disconnects from the Controly server.
	 */
	public disconnect(): void {
		if (this.ws) {
			this.ws.removeEventListener('open', this.handleOpen)
			this.ws.removeEventListener('message', this.handleMessage)
			this.ws.removeEventListener('error', this.handleError)
			this.ws.removeEventListener('close', this.handleClose)
			if (this.ws.readyState === WebSocket.OPEN) {
				this.ws.close()
			}
			this.ws = null
		}
	}

	/**
	 * Sends a message to the server.
	 * @param message The message object to send.
	 * @throws {Error} if the WebSocket is not connected.
	 */
	protected sendMessage<T extends MessageType, P>(message: IncomingMessage<T, P>): void {
		if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
			throw new Error('WebSocket is not connected. Cannot send message.')
		}
		this.ws.send(JSON.stringify(message))
	}

	private handleOpen = (): void => {
		// The 'open' event is fired after the server assigns an ID via 'set_id' message.
		console.log('WebSocket connection established. Waiting for client ID.')
	}

	private handleMessage = (event: MessageEvent): void => {
		try {
			const message = JSON.parse(event.data) as OutgoingMessage<any, any>

			if (message.type === 'set_id') {
				const { id } = message.payload as SetIdPayload
				this.clientId = id
				this.emitter.emit('open', this.clientId)
				return
			}

			if (message.type === 'error') {
				this.emitter.emit('error', message.payload as ErrorPayload, message.from)
				return
			}

			this.processMessage(message)
		} catch (error) {
			console.error('Failed to parse server message:', event.data, error)
			const syntheticError: ErrorPayload = {
				code: -1,
				message: 'Failed to parse server message',
			}
			this.emitter.emit('error', syntheticError, undefined)
		}
	}

	/**
	 * Abstract method for subclasses to process specific message types.
	 * @param message The parsed message from the server.
	 */
	protected abstract processMessage(message: OutgoingMessage<any, any>): void

	private handleError = (event: Event): void => {
		console.error('WebSocket error:', event)
		const errorPayload: ErrorPayload = {
			code: 5000,
			message: 'A WebSocket communication error occurred.',
		}
		this.emitter.emit('error', errorPayload, undefined)
	}

	private handleClose = (): void => {
		this.emitter.emit('close' as any)
	}

	/**
	 * Gets the current client ID.
	 * @returns The client ID, or null if not yet assigned.
	 */
	public getId(): string | null {
		return this.clientId
	}
}
