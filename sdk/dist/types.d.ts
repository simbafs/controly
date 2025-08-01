/**
 * @file Defines the core data structures and types used in the Controly SDK.
 */
/**
 * Represents the type of a WebSocket message.
 */
export type MessageType = 'set_id' | 'command_list' | 'command' | 'status' | 'subscribe' | 'unsubscribe' | 'notification' | 'error' | 'subscribed' | 'unsubscribed' | 'waiting';
/**
 * Base interface for all WebSocket messages.
 */
export interface BaseMessage<T extends MessageType, P> {
    type: T;
    payload: P;
}
/**
 * Interface for messages sent from the client to the server.
 */
export interface IncomingMessage<T extends MessageType, P> extends BaseMessage<T, P> {
    to?: string;
}
/**
 * Interface for messages received from the server.
 */
export interface OutgoingMessage<T extends MessageType, P> extends BaseMessage<T, P> {
    from?: string;
}
/**
 * Payload for the `set_id` message from the server.
 */
export interface SetIdPayload {
    id: string;
}
/**
 * Payload for the `subscribe` message sent by a Controller.
 */
export interface SubscribePayload {
    display_ids: string[];
}
/**
 * Payload for the `unsubscribe` message sent by a Controller.
 */
export interface UnsubscribePayload {
    display_ids: string[];
}
/**
 * Payload for a `command` message sent by a Controller.
 */
export interface CommandPayload {
    name: string;
    args?: Record<string, any>;
}
/**
 * A generic payload for `status` messages sent by a Display.
 */
export type StatusPayload = Record<string, any>;
/**
 * A generic payload for `notification` messages sent by the server.
 */
export interface NotificationPayload {
    message: string;
    [key: string]: any;
}
/**
 * Payload for `error` messages sent by the server.
 */
export interface ErrorPayload {
    code: number | string;
    message: string;
}
/**
 * Payload for the 'subscribed' message from the server.
 */
export interface SubscribedPayload {
    count: number;
}
/**
 * Payload for the 'unsubscribed' message from the server.
 */
export interface UnsubscribedPayload {
    count: number;
}
/**
 * Payload for the 'display_disconnected' message from the server.
 */
export interface DisplayDisconnectedPayload {
    display_id: string;
}
/**
 * Base interface for all command definitions.
 */
interface CommandBase {
    name: string;
    label: string;
}
export interface ButtonCommand extends CommandBase {
    type: 'button';
}
export interface TextCommand extends CommandBase {
    type: 'text';
    default?: string;
    regex?: string;
}
export interface NumberCommand extends CommandBase {
    type: 'number';
    default?: number;
    min?: number;
    max?: number;
    step?: number;
}
export interface SelectCommand extends CommandBase {
    type: 'select';
    options: {
        label: string;
        value: string | number;
    }[];
    default?: string | number;
}
export interface CheckboxCommand extends CommandBase {
    type: 'checkbox';
    default?: boolean;
}
/**
 * A union of all possible command types defined in `command.json`.
 */
export type Command = ButtonCommand | TextCommand | NumberCommand | SelectCommand | CheckboxCommand;
/**
 * Payload for the `command_list` message from the server.
 */
export type CommandListPayload = Command[];
/**
 * Generic handler for events that carry a payload and an optional source ID.
 */
export type ControlyEventHandler<T> = (payload: T, from?: string) => void;
/**
 * Handler for the 'open' event, receiving the client's assigned ID.
 */
export type OpenHandler = (id: string) => void;
/**
 * Handler for the 'close' event.
 */
export type CloseHandler = (event: CloseEvent) => void;
/**
 * Handler for 'error' events.
 */
export type ErrorHandler = ControlyEventHandler<ErrorPayload>;
/**
 * Handler for 'subscribed' events.
 */
export type SubscribedHandler = ControlyEventHandler<SubscribedPayload>;
/**
 * Handler for 'unsubscribed' events.
 */
export type UnsubscribedHandler = ControlyEventHandler<UnsubscribedPayload>;
/**
 * Handler for 'status' events from a Display.
 */
export type StatusHandler = ControlyEventHandler<StatusPayload>;
/**
 * Handler for 'command_list' events from a Display.
 */
export type CommandListHandler = ControlyEventHandler<CommandListPayload>;
/**
 * Handler for 'notification' events from the server.
 */
export type NotificationHandler = ControlyEventHandler<NotificationPayload>;
/**
 * Handler for 'display_disconnected' events from the server.
 */
export type DisplayDisconnectedHandler = (displayId: string) => void;
/**
 * Handler for 'waiting' events from the server, receiving the list of display IDs being waited for.
 */
export type WaitingHandler = (waitingList: string[]) => void;
/**
 * Handler for a specific command from a Controller.
 * @template T - The type of the command arguments.
 */
export type CommandHandler<T extends Record<string, any> = Record<string, any>> = (args: T, fromControllerId: string) => void;
/**
 * A map of all possible events and their corresponding handler types for the Controller.
 */
export interface ControllerEventMap {
    open: OpenHandler;
    close: CloseHandler;
    error: ErrorHandler;
    status: StatusHandler;
    command_list: CommandListHandler;
    notification: NotificationHandler;
    display_disconnected: DisplayDisconnectedHandler;
    waiting: WaitingHandler;
    [key: string]: (...args: any[]) => void;
}
/**
 * A map of all possible events and their corresponding handler types for the Display.
 */
export interface DisplayEventMap {
    open: OpenHandler;
    close: CloseHandler;
    error: ErrorHandler;
    subscribed: SubscribedHandler;
    unsubscribed: UnsubscribedHandler;
    [key: string]: (...args: any[]) => void;
}
/**
 * Options for initializing a Controly client.
 */
export interface ControlyOptions {
    /**
     * The WebSocket URL of the relay server.
     * @example 'wss://controly.1li.tw/ws'
     */
    serverUrl: string;
    /**
     * An optional authentication token.
     */
    token?: string;
    /**
     * An optional client ID to resume a previous session.
     */
    id?: string;
    /**
     * For Displays, the URL to the `command.json` file.
     * @example 'http://localhost:3000/command.json'
     */
    commandUrl?: string;
    /**
     * Whether to automatically reconnect on unexpected disconnection.
     * @default true
     */
    reconnect?: boolean;
    /**
     * Maximum number of reconnection attempts.
     * @default 5
     */
    maxRetries?: number;
    /**
     * Delay between reconnection attempts in milliseconds.
     * @default 2000
     */
    reconnectDelay?: number;
    /**
     * If true, suppresses all `console.log` and `console.warn` messages.
     * @default false
     */
    silent?: boolean;
}
export {};
