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
	if (id) url.searchParams.set(type === 'display' ? 'id' : 'target_id', id)

	const logPrefix = chalk.cyan(`[${clientName}]`)
	const conn = new ws(url)
	const dataQueue = []
	const resQueue = []

	conn.on('open', () => console.log(`${logPrefix} Connected to ${url.href}`))
	conn.on('error', error => {
		throw new Error(`${logPrefix} Error: ${error.message}`)
	})
	conn.on('close', () => console.log(`${logPrefix} Connection closed`))
	conn.on('message', data => {
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
		send(type, payload) {
			const message = JSON.stringify({ type, payload })
			conn.send(message, error => {
				if (error) throw new Error(`${logPrefix} Send Error: ${error.message}`)
				console.log(`${logPrefix} Sent: ${chalk.grey(message)}`)
			})
		},
		close: () => conn.close(),
	}
}

// --- Test Case Runner ---
async function testCase(name, fn) {
	testState.total++
	testState.tests.push({ name, status: 'running' })
	console.log(chalk.blue(`\n--- Running Test: ${name} ---`))
	try {
		await fn()
		testState.passed++
		testState.tests[testState.total - 1].status = 'passed'
		console.log(chalk.green(`✅ Test Passed: ${name}`))
	} catch (error) {
		testState.failed++
		testState.tests[testState.total - 1].status = 'failed'
		console.error(chalk.red(`❌ Test Failed: ${name}`))
		console.error(chalk.red.italic(error.stack))
	}
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

// --- Main Test Execution ---
async function main() {
	await testCase('Basic Connection and Command Forwarding', async () => {
		const display1 = connectWebSocket('Display 1', 'display', {
			command: `http://localhost:8081/commands/1.json`,
		})

		const setIdMsg = await display1.next()
		assert.strictEqual(setIdMsg.type, 'set_id', 'Display should receive set_id message')
		const displayID = setIdMsg.payload.id
		assert.ok(typeof displayID === 'string' && displayID.length > 0, 'Display ID should be a non-empty string')

		const controller1 = connectWebSocket('Controller 1', 'controller', { id: displayID })

		const commandListMsg = await controller1.next()
		assert.strictEqual(commandListMsg.type, 'command_list', 'Controller should receive command_list')
		assert.deepStrictEqual(commandListMsg.payload[0].name, 'play_pause', 'Command list should be correct')

		const testCommand = { name: 'test command', type: 'button' }
		controller1.send('command', testCommand)

		const forwardedCommand = await display1.next()
		assert.strictEqual(forwardedCommand.type, 'command', 'Display should receive a command')
		assert.deepStrictEqual(forwardedCommand.payload, testCommand, 'Forwarded command should match sent command')

		const testStatus = { time: 100 }
		display1.send('status', testStatus)

		const forwardedStatus = await controller1.next()
		assert.strictEqual(forwardedStatus.type, 'status', 'Controller should receive a status')
		assert.deepStrictEqual(forwardedStatus.payload, testStatus, 'Forwarded status should match sent status')

		display1.close()
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

	// Add more test cases here...
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
