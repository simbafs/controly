# Controly SDK

For more detailed project specifications and server-side implementation, please refer to the main repository: [https://github.com/simbafs/controly](https://github.com/simbafs/controly)

This SDK simplifies the integration of clients (both "Displays" and "Controllers") with the Controly relay server. It abstracts the underlying WebSocket communication, providing a high-level, event-driven API.

## Core Concepts

Imagine a scenario where you have a web page displaying something, but you want to control it from another device using a control panel. Controly is designed for this exact situation.

You create your **Display Page** (the page to be controlled), define the available actions in a `command.json` file, and use the Controly SDK to connect to a generic **Server**. The server then automatically generates a **Control Page** for you based on your `command.json`.

The key advantages of this architecture are:

- **Universal Server**: The server is generic. It can generate different control panels simply by being provided with different `command.json` files from various Display Pages.
- **Static Display Pages**: Since the Display Page itself doesn't need a backend, it's fully static. You can host it anywhere you can host a static website (like GitHub Pages, Cloudflare Pages, Vercel, etc.).
- **Simple SDK**: The SDK provides a straightforward way to connect to the server and handle commands, letting you focus on the logic of your display.

## Installation

To use the SDK in a browser environment, include it via a `<script>` tag.

```html
<script src="https://<your-cdn>/controly-sdk.js"></script>
```

---

## `controly.Display`

The `Display` class is used for clients that need to be remotely controlled. It handles connecting to the server, registering itself, receiving commands, and reporting its status.

### Usage Example

```javascript
// 1. Create a Display instance
const display = new controly.Display({
	serverUrl: 'ws://localhost:8080/ws',
	id: 'my-unique-display-01', // Optional, the server will assign one if not provided
	commandUrl: 'https://example.com/commands.json', // URL to your command definition file
})

// 2. Register command handlers
// This function will be called when a Controller sends a 'play_pause' command.
display.command('play_pause', (command, fromControllerId) => {
	console.log(`Received command '${command.name}' from controller '${fromControllerId}'`)
	// Execute your play/pause logic here...

	// After execution, update the status to notify all subscribers.
	display.updateStatus({
		playback: 'playing',
		timestamp: Date.now(),
	})
})

// Register another command handler for 'set_volume'
display.command('set_volume', (command, fromControllerId) => {
	const volumeLevel = command.args.level
	console.log(`Setting volume to ${volumeLevel} as requested by ${fromControllerId}`)
	// Execute volume setting logic...

	// Update status
	display.updateStatus({
		volume: volumeLevel,
	})
})

// 3. Listen for connection events
display.on('open', id => {
	console.log(`Display connected to server with ID: ${id}`)
})

display.on('error', error => {
	console.error('An error occurred:', error.message)
})

display.on('close', () => {
	console.log('Connection closed.')
})

// 4. Connect to the server
display.connect()

// 5. Proactively update status
// You can send status updates at any time, e.g., when a user interacts with the display locally.
setInterval(() => {
	display.updateStatus({
		heartbeat: Date.now(),
	})
}, 30000)
```

### API

#### `new controly.Display(options)`

Creates a new Display instance.

- `options.serverUrl` (string, **required**): The WebSocket URL of the relay server.
- `options.id` (string, optional): A specific ID for the Display.
- `options.commandUrl` (string, **required**): The URL of the `command.json` file.

#### `.connect()`

Establishes the connection to the server.

#### `.disconnect()`

Closes the connection.

#### `.on(eventName, callback)`

Registers an event listener.

- `eventName` (string): The event to listen for. Can be `open`, `close`, `error`, `subscribed`, `unsubscribed`.
- `callback(payload, fromId)`: The function to execute when the event is triggered.

#### `.command(commandName, callback)`

Registers a handler for a specific command.

- `commandName` (string): The name of the command.
- `callback(args, fromID)`: The function to execute when the command is received.

#### `.updateStatus(statusPayload)`

Broadcasts the current status to all subscribed Controllers.

- `statusPayload` (object): Any JSON-serializable object representing the display's state.

#### `.subscribers()`

Returns the current number of Controllers subscribed to this Display.

- **Returns**: `number`

---

## `controly.Controller`

The `Controller` class is used for clients that need to control one or more Displays. It handles connecting to the server, subscribing to Displays, receiving their command lists and status updates, and sending commands.

### Usage Example

```javascript
// 1. Create a Controller instance
const controller = new controly.Controller({
	serverUrl: 'ws://localhost:8080/ws',
	id: 'my-remote-controller-A', // Optional
})

// 2. Listen for events
controller.on('open', id => {
	console.log(`Controller connected with ID: ${id}`)
	// Once connected, subscribe to the displays you want to control.
	controller.subscribe(['display-01', 'display-02'])
})

// Fired when a subscription is successful and the Display's command list is received.
controller.on('command_list', data => {
	const { from, payload } = data
	console.log(`Received command list from Display '${from}':`, payload)
	// You can now dynamically generate a UI based on the available commands.
	// e.g., renderButtons(from, payload);
})

// Listen for status updates from Displays.
controller.on('status', data => {
	const { from, payload } = data
	console.log(`Status update from Display '${from}':`, payload)
	// e.g., updateUI(from, payload);
})

controller.on('notification', notification => {
	console.info('Server notification:', notification.message)
})

controller.on('error', error => {
	console.error('An error occurred:', error.message)
})

// 3. Connect to the server
controller.connect()

// 4. Send commands (e.g., in response to user interaction)
function sendPlayCommand(displayId) {
	controller.sendCommand(displayId, {
		name: 'play_pause',
	})
}

function sendVolumeCommand(displayId, volume) {
	controller.sendCommand(displayId, {
		name: 'set_volume',
		args: { level: volume },
	})
}
```

### API

#### `new controly.Controller(options)`

Creates a new Controller instance.

- `options.serverUrl` (string, **required**): The WebSocket URL of the relay server.
- `options.id` (string, optional): A specific ID for the Controller.

#### `.connect()`

Establishes the connection to the server.

#### `.disconnect()`

Closes the connection.

#### `.on(eventName, callback)`

Registers an event listener.

- `eventName`: `open`, `close`, `error`, `command_list`, `status`, `notification`.
- `callback(payload)`: The function to execute when the event is triggered.

#### `.subscribe(displayIds)`

Subscribes to one or more Displays.

- `displayIds` (string[]): An array of target Display IDs.

#### `.unsubscribe(displayIds)`

Unsubscribes from one or more Displays.

- `displayIds` (string[]): An array of target Display IDs.

#### `.sendCommand(displayId, command)`

Sends a command to a specific Display.

- `displayId` (string): The ID of the target Display.
- `command` (object): The command object, containing a `name` and an optional `args` object.
