import yargs from 'yargs'
import { hideBin } from 'yargs/helpers'
import { Controller, Display } from 'controly'
import express from 'express'
import cors from 'cors'
import path from 'path'
import pidusage from 'pidusage'
import chalk from 'chalk'

// --- Static Server for command.json ---
const app = express()
const staticServerPort = 3000
app.use(cors())
app.use(express.static(path.join(import.meta.dirname, '..', 'load-test')))
app.listen(staticServerPort, () => {
	console.log(chalk.dim(`Static server for command.json listening on http://localhost:${staticServerPort}`))
})
const commandUrl = `http://localhost:${staticServerPort}/command.json`

// --- Helper Types ---
interface Metrics {
	cpu: number
	memory: number
}

type MonitoredController = Controller & { hasFailed?: boolean }
type MonitoredDisplay = Display & { hasFailed?: boolean }

interface ClientPool {
	controllers: MonitoredController[]
	displays: MonitoredDisplay[]
}

interface LevelResult {
	success: boolean
	reason?: string
	avgLatency?: number
	avgCpu?: number
	avgMemory?: number
	errorRate?: number
}

// --- Configuration ---
const argvPromise = yargs(hideBin(process.argv))
	.option('pid', {
		type: 'number',
		description: 'PID of the server process to monitor',
		demandOption: true,
	})
	.option('minControllers', {
		type: 'number',
		description: 'Initial lower bound for the binary search',
		default: 1,
	})
	.option('maxControllers', {
		type: 'number',
		description: 'Initial upper bound for the binary search',
		default: 500,
	})
	.option('stepSize', {
		type: 'number',
		description: 'The precision of the search. The test stops when (high - low) <= stepSize.',
		default: 10,
	})
	.option('dpc', {
		alias: 'displays-per-controller',
		type: 'number',
		description: 'Number of Display clients per Controller',
		default: 1,
	})
	.option('cpm', {
		alias: 'commands-per-minute',
		type: 'number',
		description: 'Commands sent per minute by each Controller',
		default: 60,
	})
	.option('spm', {
		alias: 'status-updates-per-minute',
		type: 'number',
		description: 'Status updates sent per minute by each Display',
		default: 60,
	})
	.option('serverUrl', {
		type: 'string',
		description: 'WebSocket URL of the Controly server',
		default: 'ws://localhost:8080/ws',
	})
	.option('duration', {
		type: 'number',
		description: 'Duration (in seconds) for each test level',
		default: 30,
	})
	.option('maxLatency', {
		type: 'number',
		description: 'Maximum acceptable average command latency (ms)',
		default: 500,
	})
	.option('maxCpu', {
		type: 'number',
		description: 'Maximum acceptable average server CPU usage (%)',
		default: 90,
	})
	.option('maxMemory', {
		type: 'number',
		description: 'Maximum acceptable average server memory usage (MB)',
		default: 1024,
	})
	.option('maxErrorRate', {
		type: 'number',
		description: 'Maximum acceptable client error rate (0.0 to 1.0)',
		default: 0.05,
	})
	.option('creationTimeout', {
		type: 'number',
		description: 'Maximum time (in seconds) to wait for client creation in a single level.',
		default: 120,
	}).argv

// --- Global State ---
const clientPool: ClientPool = { controllers: [], displays: [] }
const latencyReadings: number[] = []

// --- Client Simulation & Management ---

async function createAndConnectController(
	controllerId: string,
	displayIds: string[],
	serverUrl: string,
	cpm: number,
): Promise<MonitoredController> {
	let isConnected = false
	const controller: MonitoredController = new Controller({ serverUrl, id: controllerId, silent: true })
	controller.hasFailed = false

	controller.on('open', () => {
		isConnected = true
		controller.subscribe(displayIds)
	})
	controller.on('error', () => (controller.hasFailed = true))
	controller.on('close', event => {
		isConnected = false
		if (event.code !== 1000) {
			controller.hasFailed = true
		}
	})
	controller.on('status', payload => {
		if (payload.pong && payload.sentTime) {
			latencyReadings.push(Date.now() - payload.sentTime)
		}
	})

	controller.connect()

	const baseInterval = 60000 / cpm
	const scheduleCommand = () => {
		if (!isConnected || controller.hasFailed) return

		const targetDisplay = displayIds[Math.floor(Math.random() * displayIds.length)]
		try {
			controller.sendCommand(targetDisplay, {
				name: 'ping',
				args: { sentTime: Date.now() },
			})
		} catch (err) {
			// Errors are handled by the 'error'/'close' listeners
		}

		const delay = baseInterval * (0.75 + Math.random() * 0.5)
		setTimeout(scheduleCommand, delay)
	}

	setTimeout(scheduleCommand, baseInterval * Math.random())
	return controller
}

async function createAndConnectDisplay(displayId: string, serverUrl: string, spm: number): Promise<MonitoredDisplay> {
	let isConnected = false
	const display: MonitoredDisplay = new Display({ serverUrl, id: displayId, commandUrl, silent: true })
	display.hasFailed = false

	display.on('open', () => (isConnected = true))
	display.on('error', () => (display.hasFailed = true))
	display.on('close', event => {
		isConnected = false
		if (event.code !== 1000) {
			display.hasFailed = true
		}
	})
	display.command('ping', ({ sentTime }) => {
		if (isConnected) {
			display.updateStatus({ pong: true, sentTime })
		}
	})

	display.connect()

	const baseInterval = 60000 / spm
	const scheduleStatusUpdate = () => {
		if (!isConnected || display.hasFailed) return

		try {
			display.updateStatus({ timestamp: Date.now() })
		} catch (err) {
			// Errors are handled by the 'error'/'close' listeners
		}

		const delay = baseInterval * (0.75 + Math.random() * 0.5)
		setTimeout(scheduleStatusUpdate, delay)
	}

	setTimeout(scheduleStatusUpdate, baseInterval * Math.random())
	return display
}

const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))

const timeout = (ms: number, message: string) =>
	new Promise((_, reject) => {
		setTimeout(() => {
			reject(new Error(message))
		}, ms)
	})

async function setClientPoolSize(
	targetControllers: number,
	dpc: number,
	serverUrl: string,
	cpm: number,
	spm: number,
	creationTimeout: number,
): Promise<boolean> {
	const currentControllers = clientPool.controllers.length
	const diff = targetControllers - currentControllers

	if (diff > 0) {
		console.log(chalk.dim(`Adding ${diff} controllers and ${diff * dpc} displays...`))

		const createClientsTask = async () => {
			for (let i = 0; i < diff; i++) {
				const controllerIdNum = clientPool.controllers.length
				const controllerId = `load-test-controller-${controllerIdNum}`
				const displayIds = Array.from({ length: dpc }, (_, j) => `load-test-display-${controllerIdNum}-${j}`)

				const controllerPromise = createAndConnectController(controllerId, displayIds, serverUrl, cpm)
				const displayPromises = displayIds.map(did => createAndConnectDisplay(did, serverUrl, spm))

				const [newController, ...newDisplays] = await Promise.all([controllerPromise, ...displayPromises])

				clientPool.controllers.push(newController as MonitoredController)
				clientPool.displays.push(...(newDisplays as MonitoredDisplay[]))

				await delay(10 + Math.random() * 20)
			}
		}

		try {
			await Promise.race([
				createClientsTask(),
				timeout(creationTimeout * 1000, `Client creation timed out after ${creationTimeout}s.`),
			])
		} catch (error: any) {
			console.log(chalk.yellow(error.message))
			return false // Indicate failure
		}
	} else if (diff < 0) {
		const toRemove = -diff
		console.log(chalk.dim(`Removing ${toRemove} controllers and ${toRemove * dpc} displays...`))
		const controllersToRemove = clientPool.controllers.splice(currentControllers - toRemove, toRemove)
		const displaysToRemove = clientPool.displays.splice((currentControllers - toRemove) * dpc, toRemove * dpc)
		controllersToRemove.forEach(c => c.disconnect())
		displaysToRemove.forEach(d => d.disconnect())
	}

	// Reset failure status for the new level
	clientPool.controllers.forEach(c => (c.hasFailed = false))
	clientPool.displays.forEach(d => (d.hasFailed = false))
	return true // Indicate success
}

// --- Monitoring ---

async function fetchServerMetrics(pid: number): Promise<Metrics | null> {
	try {
		return await pidusage(pid)
	} catch (error) {
		return null
	}
}

// --- Test Execution ---

async function runLevelForDuration(
	duration: number,
	pid: number,
	maxCpu: number,
	maxMemory: number,
	maxLatency: number,
	maxErrorRate: number,
): Promise<LevelResult> {
	return new Promise(async resolve => {
		const metricReadings: Metrics[] = []
		latencyReadings.length = 0
		let monitoringError = false

		const monitoringInterval = setInterval(async () => {
			const metrics = await fetchServerMetrics(pid)
			if (metrics) {
				metricReadings.push(metrics)
			} else {
				monitoringError = true
				clearInterval(monitoringInterval)
				resolve({ success: false, reason: `Could not monitor PID ${pid}.` })
			}
		}, 1000)

		setTimeout(() => {
			if (monitoringError) return
			clearInterval(monitoringInterval)

			if (metricReadings.length < duration / 2) {
				return resolve({ success: false, reason: 'Could not fetch enough server metrics.' })
			}

			const failedControllers = clientPool.controllers.filter(c => c.hasFailed).length
			const failedDisplays = clientPool.displays.filter(d => d.hasFailed).length
			const totalClients = clientPool.controllers.length + clientPool.displays.length
			const failedCount = failedControllers + failedDisplays
			const errorRate = totalClients > 0 ? failedCount / totalClients : 0
			const avgCpu = metricReadings.reduce((sum, m) => sum + m.cpu, 0) / metricReadings.length
			const avgMemory = metricReadings.reduce((sum, m) => sum + m.memory, 0) / metricReadings.length / 1024 / 1024
			const avgLatency =
				latencyReadings.length > 0 ? latencyReadings.reduce((sum, l) => sum + l, 0) / latencyReadings.length : 0

			if (errorRate > maxErrorRate) {
				return resolve({
					success: false,
					reason: `Error rate limit exceeded (${chalk.red((errorRate * 100).toFixed(2) + '%')} > ${(maxErrorRate * 100).toFixed(2)}%)`,
					errorRate,
				})
			}
			if (avgCpu > maxCpu) {
				return resolve({
					success: false,
					reason: `CPU limit exceeded (${chalk.red(avgCpu.toFixed(2) + '%')} > ${maxCpu}%)`,
					avgCpu,
				})
			}
			if (avgMemory > maxMemory) {
				return resolve({
					success: false,
					reason: `Memory limit exceeded (${chalk.red(avgMemory.toFixed(2) + 'MB')} > ${maxMemory}MB)`,
					avgMemory,
				})
			}
			if (avgLatency > maxLatency) {
				return resolve({
					success: false,
					reason: `Latency limit exceeded (${chalk.red(avgLatency.toFixed(2) + 'ms')} > ${maxLatency}ms)`,
					avgLatency,
				})
			}

			resolve({ success: true, avgLatency, avgCpu, avgMemory, errorRate })
		}, duration * 1000)
	})
}

async function runTestAtLevel(
	numControllers: number,
	argv: any,
): Promise<{ result: LevelResult; connections: number }> {
	const connections = {
		controllers: numControllers,
		displays: numControllers * argv.dpc,
	}
	console.log(
		`\n--- Testing with ${chalk.bold.cyan(connections.controllers.toString())} Controllers and ${chalk.bold.cyan(connections.displays.toString())} Displays ---`,
	)

	const creationSuccess = await setClientPoolSize(
		connections.controllers,
		argv.dpc,
		argv.serverUrl,
		argv.cpm,
		argv.spm,
		argv.creationTimeout,
	)

	if (!creationSuccess) {
		const reason = `Client creation timed out after ${argv.creationTimeout}s.`
		const result: LevelResult = {
			success: false,
			reason: reason,
		}
		console.error(`--- Level ${chalk.bold.red('FAILED')} ---`)
		console.error(`Reason: ${reason}`)
		return { result, connections: connections.controllers }
	}

	console.log(chalk.dim(`Stabilizing and monitoring for ${argv.duration} seconds...`))
	const result = await runLevelForDuration(
		argv.duration,
		argv.pid,
		argv.maxCpu,
		argv.maxMemory,
		argv.maxLatency,
		argv.maxErrorRate,
	)

	if (result.success) {
		console.log(`--- Level ${chalk.bold.green('PASSED')} ---`)
		console.log(
			`Avg Latency: ${chalk.green(result.avgLatency?.toFixed(2) + 'ms')}, Avg CPU: ${chalk.green(result.avgCpu?.toFixed(2) + '%')}, Avg Memory: ${chalk.green(result.avgMemory?.toFixed(2) + 'MB')}, Error Rate: ${chalk.green(((result.errorRate || 0) * 100).toFixed(2) + '%')}`,
		)
	} else {
		console.error(`--- Level ${chalk.bold.red('FAILED')} ---`)
		console.error(`Reason: ${result.reason}`)
	}

	return { result, connections: connections.controllers }
}

async function runLoadTest() {
	const argv = await argvPromise
	console.log(chalk.bold.magenta('--- Starting Controly Load Test (Binary Search Mode) ---'))
	console.log(chalk.dim('Parameters:'), argv)

	let low = argv.minControllers
	let high = argv.maxControllers
	let lastKnownGood = 0

	const minTest = await runTestAtLevel(low, argv)
	if (!minTest.result.success) {
		console.error(chalk.bold.red(`\n--- Test Complete ---`))
		console.error(chalk.red(`The server cannot even handle the minimum of ${low} controllers.`))
		process.exit(1)
	}
	lastKnownGood = low

	let highTest = await runTestAtLevel(high, argv)
	while (highTest.result.success) {
		low = high
		lastKnownGood = high
		console.log(chalk.yellow(`Upper bound of ${high} passed. Doubling to find failure point...`))
		high *= 2
		highTest = await runTestAtLevel(high, argv)
	}

	console.log(chalk.bold.magenta(`\n--- Starting Binary Search between ${low} and ${high} controllers ---`))
	while (high - low > argv.stepSize) {
		const mid = Math.floor((low + high) / 2)
		if (mid <= low) break

		const midTest = await runTestAtLevel(mid, argv)

		if (midTest.result.success) {
			low = mid
			lastKnownGood = mid
		} else {
			high = mid
		}
	}

	console.log(chalk.bold.magenta(`\n--- Test Complete ---`))
	console.log(
		`Search finished. The maximum stable load is approximately ${chalk.bold.green(lastKnownGood.toString())} controllers.`,
	)
	console.log(`This supports ${chalk.bold.cyan((lastKnownGood * argv.dpc).toString())} display clients.`)

	await setClientPoolSize(0, argv.dpc, argv.serverUrl, argv.cpm, argv.spm, argv.creationTimeout)
	process.exit(0)
}

runLoadTest().catch(err => {
	console.error(chalk.red('An unexpected error occurred:'), err)
	process.exit(1)
})
