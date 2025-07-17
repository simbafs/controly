const { setTimeout: sleep } = require('timers/promises')
const ws = require('ws')
const http = require('http')
const fs = require('fs')
const path = require('path')
const assert = require('assert')
const chalk = require('chalk')

const RELAY_SERVER_URL = 'ws://localhost:8080/ws'
const MOCK_HTTP_PORT = 8081

// --- Test Runner State ---
const testState = {
	total: 0,
	passed: 0,
	failed: 0,
	tests: [],
}

// --- Mock HTTP Server ---
const httpServer = http.createServer((req, res) => {
	const logPrefix = chalk.yellow('[HTTP Server]')
	console.log(`${logPrefix} Request for ${req.url}`)
	const filePath = req.url.endsWith('1.json')
		? 'mock_commands_1.json'
		: req.url.endsWith('2.json')
			? 'mock_commands_2.json'
			: null

	if (!filePath) {
		res.writeHead(404)
		res.end('Not Found')
		return
	}

	fs.readFile(path.join(__dirname, filePath), (err, data) => {
		if (err) {
			res.writeHead(500)
			res.end(`Error loading ${filePath}`)
			return
		}
		res.writeHead(200, { 'Content-Type': 'application/json' })
		res.end(data)
	})
})

// --- WebSocket Client Logic ---
function connectWebSocket(clientName, type, { id = null, command = null }) {
	const url = new URL(RELAY_SERVER_URL)
	url.searchParams.set('type', type)
	if (command) url.searchParams.set('command_url', command)
	if (id) url.searchParams.set('id', id)

	const logPrefix = chalk.cyan(`[${clientName}]`)
	const conn = new ws(url)
	const dataQueue = []
	const resQueue = []

	conn.on('open', () => console.log(`${logPrefix} Connected to ${url.href}`))
	conn.on('error', error => {
		throw new Error(`${logPrefix} Error: ${error.message}`)
	})
	conn.on('close', () => {
		console.log(`${logPrefix} Connection closed`)
	})
	conn.on('message', data => {
		console.log('`' + data.toString() + '`')
		const d = JSON.parse(data)
		console.log(`${logPrefix} Received: ${chalk.grey(JSON.stringify(d))}`)
		if (resQueue.length > 0) {
			resQueue.shift()(d)
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
		send(type, payload, to = null) {
			const messageObject = { type, payload, to }
			const message = JSON.stringify(messageObject)
			conn.send(message, error => {
				if (error) throw new Error(`${logPrefix} Send Error: ${error.message}`)
				console.log(`${logPrefix} Sent: ${chalk.grey(message)}`)
			})
		},
		close: () => conn.close(),
	}
}

function timeout(name, time) {
	return new Promise((_, rej) => {
		setTimeout(() => rej(new Error(`test ${name} exceed ${time} seconds`)), time * 1000)
	})
}

let testnumber = 1

// --- Test Case Runner ---
async function testCase(name, fn, time = 10) {
	testState.total++
	testState.tests.push({ name, status: 'running' })
	console.log(chalk.blue(`\n--- Running Test ${testnumber}: ${name} ---`))
	try {
		await Promise.race([fn(), timeout(name, time)])
		testState.passed++
		testState.tests[testState.total - 1].status = 'passed'
		console.log(chalk.green(`✅ Test Passed: ${name}`))
	} catch (error) {
		testState.failed++
		testState.tests[testState.total - 1].status = 'failed'
		console.error(chalk.red(`❌ Test ${testnumber} Failed: ${name}`))
		console.error(chalk.red.italic(error.stack))
	}
	testnumber++
}

// --- Test Report ---
function printReport() {
	console.log(chalk.blue('\n\n--- Test Summary ---'))
	testState.tests.forEach(test => {
		const status = test.status === 'passed' ? chalk.green('✅ Passed') : chalk.red('❌ Failed')
		console.log(`${status}: ${test.name}`)
	})
	console.log('\n')
	console.log(chalk.green(`Passed: ${testState.passed}`))
	console.log(chalk.red(`Failed: ${testState.failed}`))
	console.log(chalk.white(`Total: ${testState.total}`))
	console.log(chalk.blue('--------------------'))
}

const API_BASE_URL = 'http://localhost:8080/api'

async function getConnections() {
	return fetch(`${API_BASE_URL}/connections`)
		.then(res => {
			if (res.status !== 200) {
				throw new Error(`Failed to get connections: ${res.status}`)
			}
			return res.json()
		})
		.then(data => {
			console.log(chalk.yellow('[API]'), `GET /connections response:`, chalk.grey(JSON.stringify(data)))
			return data
		})
}

function deleteDisplay(id) {
	return new Promise((resolve, reject) => {
		const req = http.request(`${API_BASE_URL}/displays/${id}`, { method: 'DELETE' }, res => {
			if (res.statusCode !== 204 && res.statusCode !== 200) {
				return reject(new Error(`Failed to delete display ${id}: ${res.statusCode}`))
			}
			console.log(chalk.yellow('[API]'), `DELETE /displays/${id} successful`)
			resolve()
		})
		req.on('error', reject)
		req.end()
	})
}

function deleteController(id) {
	return new Promise((resolve, reject) => {
		const req = http.request(`${API_BASE_URL}/controllers/${id}`, { method: 'DELETE' }, res => {
			if (res.statusCode !== 204 && res.statusCode !== 200) {
				return reject(new Error(`Failed to delete controller ${id}: ${res.statusCode}`))
			}
			console.log(chalk.yellow('[API]'), `DELETE /controllers/${id} successful`)
			resolve()
		})
		req.on('error', reject)
		req.end()
	})
}

// --- Main Test Execution ---
async function main() {
	await testCase('Basic Connection and Command Forwarding (One-to-One)', async () => {
		const display1 = connectWebSocket('Display 1', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})

		const setIdMsg = await display1.next()
		assert.strictEqual(setIdMsg.type, 'set_id', 'Display should receive set_id message')
		const displayID = setIdMsg.payload.id
		assert.ok(typeof displayID === 'string' && displayID.length > 0, 'Display ID should be a non-empty string')

		const controller1 = connectWebSocket('Controller 1', 'controller', {})
		const controller1IdMsg = await controller1.next()
		assert.strictEqual(controller1IdMsg.type, 'set_id', 'Controller should receive set_id message')
		const controller1ID = controller1IdMsg.payload.id

		// Controller subscribes to Display 1
		controller1.send('subscribe', { display_ids: [displayID] })

		const commandListMsg = await controller1.next()
		assert.strictEqual(commandListMsg.type, 'command_list', 'Controller should receive command_list')
		assert.strictEqual(commandListMsg.from, displayID, 'Command list should be from the correct display')
		assert.deepStrictEqual(commandListMsg.payload[0].name, 'play_pause', 'Command list should be correct')

		const testCommand = { name: 'set_volume', args: { level: 75 } }
		controller1.send('command', testCommand, displayID)

		const forwardedCommand = await display1.next()
		assert.strictEqual(forwardedCommand.type, 'command', 'Display should receive a command')
		assert.strictEqual(
			forwardedCommand.from,
			controller1ID,
			'Forwarded command should be from the correct controller',
		)
		assert.deepStrictEqual(
			forwardedCommand.payload,
			testCommand,
			'Forwarded command payload should match sent command',
		)

		const testStatus = { current_volume: 75 }
		display1.send('status', testStatus)

		const forwardedStatus = await controller1.next()
		assert.strictEqual(forwardedStatus.type, 'status', 'Controller should receive a status')
		assert.strictEqual(forwardedStatus.from, displayID, 'Forwarded status should specify source display ID')
		assert.deepStrictEqual(forwardedStatus.payload, testStatus, 'Forwarded status payload should match sent status')

		// Verify connections via REST API
		const connections = await getConnections()
		assert.ok(
			connections.displays.some(d => d.id === displayID && d.subscribers.includes(controller1ID)),
			'Display 1 should show Controller 1 as a subscriber',
		)
		assert.ok(
			connections.controllers.some(c => c.id === controller1ID && c.subscriptions.includes(displayID)),
			'Controller 1 should show Display 1 as a subscription',
		)

		display1.close()
		// await sleep(1000) // Allow time for WebSocket close to propagate
		controller1.close()
	})

	await testCase('Display with pre-defined ID', async () => {
		const customId = 'my-custom-display'
		const display = connectWebSocket('Custom ID Display', 'display', {
			id: customId,
			command: `http://localhost:8081/commands/2.json`,
		})

		const setIdMsg = await display.next()
		assert.strictEqual(setIdMsg.type, 'set_id', 'Display should receive set_id message')
		assert.strictEqual(setIdMsg.payload.id, customId, 'Server should respect the provided custom ID')

		display.close()
	})

	await testCase('Multi-to-Many Subscription and Command Forwarding', async () => {
		const display1 = connectWebSocket('Display M1', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})
		const setIdMsg1 = await display1.next()
		assert.strictEqual(setIdMsg1.type, 'set_id')
		const display1ID = setIdMsg1.payload.id

		const display2 = connectWebSocket('Display M2', 'display', {
			command: `http://localhost:8081/commands/2.json`,
		})
		const setIdMsg2 = await display2.next()
		assert.strictEqual(setIdMsg2.type, 'set_id')
		const display2ID = setIdMsg2.payload.id

		const controllerA = connectWebSocket('Controller A', 'controller', {})
		const controllerAIdMsg = await controllerA.next()
		assert.strictEqual(controllerAIdMsg.type, 'set_id')
		const controllerAID = controllerAIdMsg.payload.id

		const controllerB = connectWebSocket('Controller B', 'controller', {})
		const controllerBIdMsg = await controllerB.next()
		assert.strictEqual(controllerBIdMsg.type, 'set_id')
		const controllerBID = controllerBIdMsg.payload.id

		// Controller A subscribes to Display M1 and Display M2
		controllerA.send('subscribe', { display_ids: [display1ID, display2ID] })

		const cmdListA1 = await controllerA.next()
		assert.strictEqual(cmdListA1.type, 'command_list', 'Controller A should get cmd list for D1')
		assert.strictEqual(cmdListA1.from, display1ID, 'Command list for D1 should be from D1')
		const cmdListA2 = await controllerA.next()
		assert.strictEqual(cmdListA2.type, 'command_list', 'Controller A should get cmd list for D2')
		assert.strictEqual(cmdListA2.from, display2ID, 'Command list for D2 should be from D2')

		// Controller B subscribes to Display M1
		controllerB.send('subscribe', { display_ids: [display1ID] })

		const cmdListB1 = await controllerB.next()
		assert.strictEqual(cmdListB1.type, 'command_list', 'Controller B should get cmd list for D1')
		assert.strictEqual(cmdListB1.from, display1ID, 'Command list for D1 should be from D1')

		// Verify connections via REST API
		let connections = await getConnections()
		assert.ok(
			connections.displays.some(
				d =>
					d.id === display1ID &&
					d.subscribers.includes(controllerAID) &&
					d.subscribers.includes(controllerBID),
			),
			'Display M1 should have both A and B as subscribers',
		)
		assert.ok(
			connections.displays.some(
				d =>
					d.id === display2ID &&
					d.subscribers.includes(controllerAID) &&
					!d.subscribers.includes(controllerBID),
			),
			'Display M2 should have only A as subscriber',
		)
		assert.ok(
			connections.controllers.some(
				c =>
					c.id === controllerAID &&
					c.subscriptions.includes(display1ID) &&
					c.subscriptions.includes(display2ID),
			),
			'Controller A should be subscribed to both D1 and D2',
		)
		assert.ok(
			connections.controllers.some(
				c =>
					c.id === controllerBID &&
					c.subscriptions.includes(display1ID) &&
					!c.subscriptions.includes(display2ID),
			),
			'Controller B should be subscribed to only D1',
		)

		// Controller A sends command to Display M1
		const cmdA_D1 = { name: 'play_pause' }
		controllerA.send('command', cmdA_D1, display1ID)
		const fwdCmdA_D1 = await display1.next()
		assert.strictEqual(fwdCmdA_D1.type, 'command')
		assert.strictEqual(fwdCmdA_D1.from, controllerAID)
		assert.deepStrictEqual(fwdCmdA_D1.payload, cmdA_D1)

		// Controller B sends command to Display M1
		const cmdB_D1 = { name: 'set_volume', args: { level: 50 } }
		controllerB.send('command', cmdB_D1, display1ID)
		const fwdCmdB_D1 = await display1.next()
		assert.strictEqual(fwdCmdB_D1.type, 'command')
		assert.strictEqual(fwdCmdB_D1.from, controllerBID)
		assert.deepStrictEqual(fwdCmdB_D1.payload, cmdB_D1)

		// Display M1 sends status, both Controller A and B should receive
		const statusD1 = { status: 'online' }
		display1.send('status', statusD1)

		const fwdStatusA_D1 = await controllerA.next()
		assert.strictEqual(fwdStatusA_D1.type, 'status')
		assert.strictEqual(fwdStatusA_D1.from, display1ID)
		assert.deepStrictEqual(fwdStatusA_D1.payload, statusD1)

		const fwdStatusB_D1 = await controllerB.next()
		assert.strictEqual(fwdStatusB_D1.type, 'status')
		assert.strictEqual(fwdStatusB_D1.from, display1ID)
		assert.deepStrictEqual(fwdStatusB_D1.payload, statusD1)

		// Controller A unsubscribes from Display M2
		controllerA.send('unsubscribe', { display_ids: [display2ID] })

		// Verify connections after unsubscribe
		connections = await getConnections()
		assert.ok(
			connections.displays.some(d => d.id === display2ID && !d.subscribers?.includes(controllerAID)),
			'Display M2 should no longer have Controller A as subscriber',
		)
		assert.ok(
			connections.controllers.some(c => c.id === controllerAID && !c.subscriptions?.includes(display2ID)),
			'Controller A should no longer be subscribed to D2',
		)

		display1.close()
		display2.close()
		controllerA.close()
		controllerB.close()
	})

	await testCase('Controller Subscribing to Non-Existent Display', async () => {
		const controller = connectWebSocket('Controller NonExist', 'controller', {})
		const controllerIdMsg = await controller.next()
		assert.strictEqual(controllerIdMsg.type, 'set_id')

		controller.send('subscribe', { display_ids: ['non-existent-display'] })

		const errorMsg = await controller.next()
		assert.strictEqual(errorMsg.type, 'error', 'Should receive an error message')
		assert.strictEqual(errorMsg.payload.code, 3001, 'Error code should be ErrTargetDisplayNotFound')
		assert.ok(errorMsg.payload.message.includes('not found'), 'Error message should indicate display not found')

		controller.close()
	})

	await testCase('Display ID Conflict', async () => {
		const customId = 'conflict-display'
		const display1 = connectWebSocket('Display Conflict 1', 'display', {
			id: customId,
			command: `http://localhost:8081/commands/1.json`,
		})

		const setIdMsg1 = await display1.next()
		assert.strictEqual(setIdMsg1.type, 'set_id')
		assert.strictEqual(setIdMsg1.payload.id, customId)

		const display2 = connectWebSocket('Display Conflict 2', 'display', {
			id: customId,
			command: `http://localhost:8081/commands/2.json`,
		})

		const errorMsg = await display2.next()
		console.log(errorMsg)
		assert.strictEqual(errorMsg.type, 'error', 'Should receive an error message')
		assert.strictEqual(errorMsg.payload.code, 2003, 'Error code should be ErrDisplayIDConflict')
		assert.ok(errorMsg.payload.message.includes('already in use'), 'Error message should indicate ID conflict')

		console.log(1)
		display1.close()
		console.log(2)
		display2.close()
		console.log(3)
	})

	await testCase('Invalid Message Type Handling', async () => {
		const display = connectWebSocket('Display Invalid Msg', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})
		const setIdMsgD = await display.next()
		assert.strictEqual(setIdMsgD.type, 'set_id')
		const displayID = setIdMsgD.payload.id

		const controller = connectWebSocket('Controller Invalid Msg', 'controller', {})
		const setIdMsgC = await controller.next()
		assert.strictEqual(setIdMsgC.type, 'set_id')
		const controllerID = setIdMsgC.payload.id

		// Display sends invalid type
		display.send('invalid_type', { some: 'data' })
		const displayError = await display.next()
		assert.strictEqual(displayError.type, 'error')
		assert.strictEqual(displayError.payload.code, 4001, 'Error code should be ErrInvalidMessageFormat')

		// Controller sends invalid type
		controller.send('another_invalid_type', { some: 'other data' })
		const controllerError = await controller.next()
		assert.strictEqual(controllerError.type, 'error')
		assert.strictEqual(controllerError.payload.code, 4001, 'Error code should be ErrInvalidMessageFormat')

		display.close()
		controller.close()
	})

	await testCase('Controller Sending Command to Unsubscribed Display', async () => {
		const display1 = connectWebSocket('Display Unsub1', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})
		const setIdMsg1 = await display1.next()
		assert.strictEqual(setIdMsg1.type, 'set_id')
		const display1ID = setIdMsg1.payload.id

		const display2 = connectWebSocket('Display Unsub2', 'display', {
			command: `http://localhost:8081/commands/2.json`,
		})
		const setIdMsg2 = await display2.next()
		assert.strictEqual(setIdMsg2.type, 'set_id')
		const display2ID = setIdMsg2.payload.id

		const controller = connectWebSocket('Controller Unsub', 'controller', {})
		const setIdMsgC = await controller.next()
		assert.strictEqual(setIdMsgC.type, 'set_id')
		const controllerID = setIdMsgC.payload.id

		// Controller subscribes only to Display 2
		controller.send('subscribe', { display_ids: [display2ID] })
		await controller.next() // command_list for D2

		// Controller attempts to send command to Display 1 (unsubscribed)
		const testCommand = { name: 'play_pause' }
		controller.send('command', testCommand, display1ID)

		const errorMsg = await controller.next()
		assert.strictEqual(errorMsg.type, 'error', 'Should receive an error message')
		assert.strictEqual(errorMsg.payload.code, 3004, 'Error code should be ErrNotSubscribedToDisplay')
		assert.ok(
			errorMsg.payload.message.includes('Not subscribed to display'),
			'Error message should indicate not subscribed',
		)

		display1.close()
		display2.close()
		controller.close()
	})


	// TODO: These two test will cause deadlock
	await testCase('Delete Display via REST API', async () => {
		const display = connectWebSocket('Display Del', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})
		const setIdMsgD = await display.next()
		assert.strictEqual(setIdMsgD.type, 'set_id')
		const displayID = setIdMsgD.payload.id

		const controller = connectWebSocket('Controller Del', 'controller', {})
		const setIdMsgC = await controller.next()
		assert.strictEqual(setIdMsgC.type, 'set_id')
		const controllerID = setIdMsgC.payload.id

		controller.send('subscribe', { display_ids: [displayID] })
		await controller.next() // command_list

		let connections = await getConnections()
		assert.ok(
			connections.displays.some(d => d.id === displayID && d.subscribers.includes(controllerID)),
			'Display should be active and subscribed',
		)

		await deleteDisplay(displayID)

		// Controller should receive an error/notification about display disconnection
		const errorMsg = await controller.next()
		assert.strictEqual(errorMsg.type, 'error', 'Controller should receive error after display deletion')
		assert.strictEqual(errorMsg.payload.code, 3001, 'Error code should be ErrTargetDisplayNotFound')

		connections = await getConnections()
		assert.ok(!connections.displays.some(d => d.id === displayID), 'Display should be deleted from connections')
		assert.ok(
			connections.controllers.some(c => c.id === controllerID && !c.subscriptions.includes(displayID)),
			'Controller should no longer be subscribed to deleted display',
		)

		display.close()
		controller.close()
	})

	await testCase('Delete Controller via REST API', async () => {
		const display = connectWebSocket('Display DelCtrl', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})
		const setIdMsgD = await display.next()
		assert.strictEqual(setIdMsgD.type, 'set_id')
		const displayID = setIdMsgD.payload.id

		const controller = connectWebSocket('Controller DelCtrl', 'controller', {})
		const setIdMsgC = await controller.next()
		assert.strictEqual(setIdMsgC.type, 'set_id')
		const controllerID = setIdMsgC.payload.id

		controller.send('subscribe', { display_ids: [displayID] })
		await controller.next() // command_list

		let connections = await getConnections()
		assert.ok(
			connections.controllers.some(c => c.id === controllerID && c.subscriptions.includes(displayID)),
			'Controller should be active and subscribed',
		)

		await deleteController(controllerID)

		// Controller connection should close, leading to a read error
		await assert.rejects(
			controller.next(),
			{ message: /WebSocket is not open/ },
			'Controller connection should close',
		)

		connections = await getConnections()
		assert.ok(
			!connections.controllers.some(c => c.id === controllerID),
			'Controller should be deleted from connections',
		)
		assert.ok(
			connections.displays.some(d => d.id === displayID && !d.subscribers.includes(controllerID)),
			'Display should no longer have deleted controller as subscriber',
		)

		display.close()
		controller.close()
	})
}

// --- Server and Test Runner Startup ---
httpServer.listen(MOCK_HTTP_PORT, async () => {
	console.log(chalk.yellow(`Mock HTTP Server listening on http://localhost:${MOCK_HTTP_PORT}`))
	try {
		await main()
	} catch (e) {
		console.error(chalk.red.bold('A critical error occurred during test execution:'), e)
		testState.failed++
	} finally {
		printReport()
		httpServer.close()
		process.exit(testState.failed > 0 ? 1 : 0)
	}
})
