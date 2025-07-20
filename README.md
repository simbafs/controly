# Controly

A framework for remotely controlling a web page from another device via a dynamically generated control panel.

## Core Concepts

Imagine a scenario where you have a web page displaying something, but you want to control it from another device using a control panel. Controly is designed for this exact situation.

You create your **Display Page** (the page to be controlled), define the available actions in a `command.json` file, and use the Controly SDK to connect to a generic **Server**. The server then automatically generates a **Control Page** for you based on your `command.json`.

The key advantages of this architecture are:

- **Universal Server**: The server is generic. It can generate different control panels simply by being provided with different `command.json` files from various Display Pages.
- **Static Display Pages**: Since the Display Page itself doesn't need a backend, it's fully static. You can host it anywhere you can host a static website (like GitHub Pages, Cloudflare Pages, Vercel, etc.).
- **Simple SDK**: The SDK provides a straightforward way to connect to the server and handle commands, letting you focus on the logic of your display.

## Architecture

```
+-----------------+      +-------------------+      +-----------------+
|                 |      |                   |      |                 |
|  Control Page   |----->|  Controly Server  |----->|   Display Page  |
| (Your Browser)  |      |   (Go Backend)    |      | (e.g., on a TV) |
|                 |      |                   |      |                 |
+-----------------+      +-------------------+      +-----------------+
        |                                                |
        |                                                v
        |                               +---------------------+
        |                               |                     |
        +------------------------------>|    command.json     |
             (Generated based on)       | (Defines controls)  |
                                        |                     |
                                        +---------------------+
```

## How to Use

Creating your own remotely controlled page is straightforward. The basic steps are:

1.  **Set up a static web project**: Use a bundler like Vite.
2.  **Create `command.json`**: In a `public` directory, define the controls you want on your controller page.
3.  **Use the SDK in your code**:
    - Import and initialize the `Display` from the `controly`.
    - Set up event handlers for `open` (to get the controller ID).
    - Set command handlers with `display.command()`
    - Call `display.connect()`.
4.  **Deploy**: Build your project and host the static files anywhere.

For a complete, working example, please refer to the [countdown](./countdown) directory in this repository. It provides a clear demonstration of how to structure your project and use the SDK.

## Build Your Own Controller

While the server provides a default, universal controller that is sufficient for most scenarios, you also have the flexibility to build a completely custom controller.

The `controly` exports a `Controller` class that allows you to create your own controller interface. This gives you full control over the appearance and functionality of your remote.

For a complete implementation example, please see the code in the [`/server/controller`](./server/controller/) directory. It serves as the foundation for the default controller and is a great reference for building your own.

## Project Structure

- `/sdk`: The TypeScript SDK for the Display Page.
- `/server`: The universal Go server that handles WebSocket connections and serves the controller.
- `/countdown`: An example project demonstrating how to use the SDK to create a controllable countdown timer.

## License

This project is licensed under the [MIT License](LICENSE).
