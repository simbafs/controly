const ws = require('ws')
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
	test('display1')
		.then(() => test())
		.then(() => process.exit(0))
		.catch(console.error)
})

// --- WebSocket Client Logic ---

function connectWebSocket(type, { id = null, command = null }) {
	const url = new URL(RELAY_SERVER_URL)
	url.searchParams.set('type', type)
	if (command) url.searchParams.set('command_url', command)
	if (id) url.searchParams.set(type == 'display' ? 'id' : 'target_id', id)

	let conn

	try {
		conn = new ws(url)
	} catch (err) {
		throw err
	}

	const dataQueue = []
	const resQueue = []

	conn.on('open', () => {
		console.log(`[WebSocket] Connected to ${url.href}`)
	})

	conn.on('error', error => {
		throw new Error(`[WebSocket] Error: ${error.message}`)
	})

	conn.on('close', () => {
		console.log('[WebSocket] connection close')
	})

	conn.on('message', data => {
		const d = JSON.parse(data)
		if (resQueue.length > 0) {
			const res = resQueue.shift()
			res(d)
		} else {
			dataQueue.push(d)
		}
	})

	return {
		next() {
			return new Promise(res => {
				if (dataQueue.length > 0) {
					res(dataQueue.shift())
				} else {
					resQueue.push(res)
				}
			})
		},
		send(type, payload) {
			const message = JSON.stringify({ type, payload })
			conn.send(message, error => {
				if (error) {
					throw new Error(`[WebSocket] Send Error: ${error.message}`)
				} else {
					console.log(`[WebSocket] Sent: ${message}`)
				}
			})
		},
		close() {
			conn.close()
		},
	}
}

async function test(id = null) {
	const display1 = connectWebSocket('display', {
		id,
		command: `http://localhost:8081/commands/1.json`,
	})

	const displayID = await display1.next().then(data => data.payload.id)

	const controller1 = connectWebSocket('controller', { id: displayID })

	await controller1.next().then(data => console.log(`commands from ${displayID}: ${JSON.stringify(data, null, 2)}`))

	controller1.send('command', {
		name: 'test command',
		type: 'button',
	})

	await display1.next().then(cmd => console.log(`display1 reveived command: ${JSON.stringify(cmd, null, 2)}`))

	display1.send('status', {
		time: 100,
	})

	await controller1
		.next()
		.then(status => console.log(`controller1 reveived status: ${JSON.stringify(status, null, 2)}`))

	// display1.close()
	// controller1.close()
}
