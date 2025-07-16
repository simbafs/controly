const { setTimeout } = require('timers/promises')
const WebSocket = require('ws')
const http = require('http')
const fs = require('fs')
const path = require('path')

const RELAY_SERVER_URL = 'ws://localhost:8080/ws'
const MOCK_HTTP_PORT = 8081

// --- Mock HTTP Server to serve command.json files ---
const httpServer = http.createServer((req, res) => {
    console.log(`[HTTP Server] Request for ${req.url}`)
    if (req.url === '/commands/1.json') {
        fs.readFile(path.join(__dirname, 'mock_commands_1.json'), (err, data) => {
            if (err) {
                res.writeHead(500)
                res.end('Error loading mock_commands_1.json')
                return
            }
            res.writeHead(200, { 'Content-Type': 'application/json' })
            res.end(data)
        })
    } else if (req.url === '/commands/2.json') {
        fs.readFile(path.join(__dirname, 'mock_commands_2.json'), (err, data) => {
            if (err) {
                res.writeHead(500)
                res.end('Error loading mock_commands_2.json')
                return
            }
            res.writeHead(200, { 'Content-Type': 'application/json' })
            res.end(data)
        })
    } else {
        res.writeHead(404)
        res.end('Not Found')
    }
})

httpServer.listen(MOCK_HTTP_PORT, () => {
    console.log(`Mock HTTP Server listening on http://localhost:${MOCK_HTTP_PORT}`)
})

// --- WebSocket Client Logic ---

function createWebSocketClient(url, type, id = null) {
    return new Promise((resolve, reject) => {
        const ws = new WebSocket(url);
        let assignedId = id; // Keep track of the ID, initially null for server-assigned

        ws.onopen = () => {
            console.log(`[${type.toUpperCase()}${assignedId ? ' ' + assignedId : ''}] Connected to relay server.`);
            // For displays without a pre-assigned ID, we wait for the 'set_id' message
            if (type !== 'display' || id !== null) {
                resolve({ ws, assignedId });
            }
        };

        ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            console.log(`[${type.toUpperCase()}${assignedId ? ' ' + assignedId : ''}] Received:`, JSON.stringify(message, null, 2));

            if (message.type === 'error') {
                console.error(
                    `[${type.toUpperCase()}${assignedId ? ' ' + assignedId : ''}] Error: Code ${message.payload.code}, Message: ${message.payload.message}`,
                );
            } else if (type === 'display' && message.type === 'set_id') {
                assignedId = message.payload.id; // Server assigned ID
                console.log(`[DISPLAY ${assignedId}] Server assigned ID: ${assignedId}`);
                resolve({ ws, assignedId }); // Resolve once ID is received
            }
        };

        ws.onclose = () => {
            console.log(`[${type.toUpperCase()}${assignedId ? ' ' + assignedId : ''}] Disconnected from relay server.`);
        };

        ws.onerror = (error) => {
            console.error(`[${type.toUpperCase()}${assignedId ? ' ' + assignedId : ''}] WebSocket Error:`, error.message);
            reject(error);
        };
    });
}

async function runTests() {
    console.log('\n--- Starting Test Scenario ---')

    // --- Display 1 (Server-assigned ID) ---
    let display1Id = null; // This will be assigned by the server
    const display1CommandUrl = `http://localhost:${MOCK_HTTP_PORT}/commands/1.json`;
    let display1Ws;
    try {
        const { ws, assignedId } = await createWebSocketClient(
            `${RELAY_SERVER_URL}?type=display&command_url=${encodeURIComponent(display1CommandUrl)}`,
            'display',
            null, // No ID provided, server will assign
        );
        display1Ws = ws;
        display1Id = assignedId; // Server assigned ID
    } catch (error) {
        console.error(`Failed to connect Display 1: ${error.message}`);
        process.exit(1);
    }

    await setTimeout(1000); // Give server time to process registration

    // --- Display 2 ---
    const display2Id = 'my-display-2';
    const display2CommandUrl = `http://localhost:${MOCK_HTTP_PORT}/commands/2.json`;
    let display2Ws;
    try {
        const { ws } = await createWebSocketClient(
            `${RELAY_SERVER_URL}?type=display&id=${display2Id}&command_url=${encodeURIComponent(display2CommandUrl)}`,
            'display',
            display2Id,
        );
        display2Ws = ws;
    } catch (error) {
        console.error(`Failed to connect Display 2: ${error.message}`);
        process.exit(1);
    }

    await setTimeout(1000); // Give server time to process registration

    // --- Controller 1 (controls Display 1) ---
    let controller1Ws;
    try {
        const { ws } = await createWebSocketClient(
            `${RELAY_SERVER_URL}?type=controller&target_id=${display1Id}`,
            'controller',
            '1',
        );
        controller1Ws = ws;
    } catch (error) {
        console.error(`Failed to connect Controller 1: ${error.message}`);
        process.exit(1);
    }

    await setTimeout(1000); // Give time to receive command_list

    // Controller 1 sends a command to Display 1
    const command1 = {
        type: 'command',
        payload: {
            name: 'set_volume',
            args: { level: 75 },
        },
    };
    console.log(`[CONTROLLER 1] Sending command to ${display1Id}:`, JSON.stringify(command1, null, 2));
    controller1Ws.send(JSON.stringify(command1));

    await setTimeout(1000);

    // Display 1 sends a status update
    const status1 = {
        type: 'status',
        payload: {
            current_volume: 75,
            playback_state: 'playing',
        },
    };
    console.log(`[DISPLAY ${display1Id}] Sending status update:`, JSON.stringify(status1, null, 2));
    display1Ws.send(JSON.stringify(status1));

    await setTimeout(2000); // Wait for status to propagate

    // --- Controller 2 (controls Display 2) ---
    let controller2Ws;
    try {
        controller2Ws = await createWebSocketClient(
            `${RELAY_SERVER_URL}?type=controller&target_id=${display2Id}`,
            'controller',
            '2',
        );
    } catch (error) {
        console.error(`Failed to connect Controller 2: ${error.message}`);
        process.exit(1);
    }

    await setTimeout(1000); // Give time to receive command_list

    // Controller 2 sends a command to Display 2
    const command2 = {
        type: 'command',
        payload: {
            name: 'set_title',
            args: { title: 'Hello Controly' },
        },
    };
    console.log(`[CONTROLLER 2] Sending command to ${display2Id}:`, JSON.stringify(command2, null, 2));
    controller2Ws.send(JSON.stringify(command2));

    await setTimeout(1000);

    // Display 2 sends a status update
    const status2 = {
        type: 'status',
        payload: {
            current_title: 'Hello Controly',
            loop_enabled: false,
        },
    };
    console.log(`[DISPLAY ${display2Id}] Sending status update:`, JSON.stringify(status2, null, 2));
    display2Ws.send(JSON.stringify(status2));

    await setTimeout(2000); // Wait for status to propagate

    console.log('\n--- Test Scenario Complete ---')

    // Clean up
    display1Ws.close()
    display2Ws.close()
    controller1Ws.close()
    controller2Ws.close()
    httpServer.close(() => console.log('Mock HTTP Server closed.'))
}

runTests().catch(err => {
    console.error('An error occurred during tests:', err)
    httpServer.close(() => console.log('Mock HTTP Server closed due to error.'))
    process.exit(1)
})
