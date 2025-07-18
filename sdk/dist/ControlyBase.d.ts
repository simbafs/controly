/**
 * @file Implements the base client with common WebSocket logic for Controly.
 */
import { IncomingMessage, OutgoingMessage, MessageType } from './types';
/**
 * An internal, simple event emitter.
 * @template T - A map of event names to their handler types.
 */
declare class EventEmitter<T extends Record<string, (...args: any[]) => void>> {
    private listeners;
    /**
     * Registers an event listener.
     * @param eventName The name of the event.
     * @param callback The callback function.
     */
    on<E extends keyof T>(eventName: E, callback: T[E]): void;
    /**
     * Emits an event, calling all registered listeners.
     * @param eventName The name of the event.
     * @param args The arguments to pass to the listeners.
     */
    emit<E extends keyof T>(eventName: E, ...args: any[]): void;
}
/**
 * Abstract base class for Controly clients, handling common WebSocket functionality.
 * @template EventMap - A map of event names to their handler types.
 */
export declare abstract class ControlyBase<EventMap extends Record<string, (...args: any[]) => void>> {
    protected ws: WebSocket | null;
    protected emitter: EventEmitter<EventMap>;
    protected clientId: string | null;
    /**
     * The full WebSocket server URL.
     */
    readonly fullUrl: string;
    /**
     * Creates an instance of ControlyBase.
     * @param serverUrl The WebSocket URL of the relay server (e.g., 'ws://localhost:8080/ws').
     * @param params URL query parameters to be added to the server URL.
     */
    constructor(serverUrl: string, params: Record<string, string>);
    /**
     * Registers an event listener for a specific event.
     * @param eventName The name of the event to listen for.
     * @param callback The function to call when the event is emitted.
     */
    on<E extends keyof EventMap>(eventName: E, callback: EventMap[E]): void;
    /**
     * Establishes a connection to the Controly server.
     * @throws {Error} if the connection is already open or in the process of connecting.
     */
    connect(): void;
    /**
     * Disconnects from the Controly server.
     */
    disconnect(): void;
    /**
     * Sends a message to the server.
     * @param message The message object to send.
     * @throws {Error} if the WebSocket is not connected.
     */
    protected sendMessage<T extends MessageType, P>(message: IncomingMessage<T, P>): void;
    private handleOpen;
    private handleMessage;
    /**
     * Abstract method for subclasses to process specific message types.
     * @param message The parsed message from the server.
     */
    protected abstract processMessage(message: OutgoingMessage<any, any>): void;
    private handleError;
    private handleClose;
    /**
     * Gets the current client ID.
     * @returns The client ID, or null if not yet assigned.
     */
    getId(): string | null;
}
export {};
