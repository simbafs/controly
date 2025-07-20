import './style.css'

const SERVER_URL = import.meta.env.PROD
	? location.origin.replace('http', 'ws') + '/ws/inspector'
	: 'ws://localhost:8080/ws/inspector'

const statusEl = document.getElementById('status')
const logEntriesEl = document.getElementById('log-entries')
const sourceFilterEl = document.getElementById('source-filter') as HTMLInputElement
const targetsFilterEl = document.getElementById('targets-filter') as HTMLInputElement
const messageFilterEl = document.getElementById('message-filter') as HTMLInputElement
const maxLogInputEl = document.getElementById('max-log-input') as HTMLInputElement
const clearLogBtnEl = document.getElementById('clear-log-btn') as HTMLButtonElement

let allLogs: any[] = []

function clearLogs() {
	allLogs = []
	renderLogs()
}

function applyMaxLogLimit() {
	const maxLogs = parseInt(maxLogInputEl.value, 10)
	if (!isNaN(maxLogs) && maxLogs > 0 && allLogs.length > maxLogs) {
		allLogs = allLogs.slice(allLogs.length - maxLogs)
	}
}

function renderLogs() {
	if (!logEntriesEl) return

	const sourceFilter = sourceFilterEl.value.trim()
	const targetsFilter = targetsFilterEl.value.trim()
	const messageFilter = messageFilterEl.value.trim()

	const filteredLogs = allLogs.filter(log => {
		const sourceMatch = !sourceFilter || log.source.includes(sourceFilter)
		const targetsMatch = !targetsFilter || log.targets.some((t: string) => t.includes(targetsFilter))
		const messageMatch = !messageFilter || JSON.stringify(log.original_message).includes(messageFilter)
		return sourceMatch && targetsMatch && messageMatch
	})

	logEntriesEl.innerHTML = '' // Clear existing logs

	filteredLogs.forEach(data => {
		const row = document.createElement('div')
		row.className =
			'log-row grid grid-cols-12 gap-4 p-4 border-b border-gray-700 text-sm hover:bg-gray-700/50 transition-colors duration-150 ease-in-out'

		// 1. Timestamp
		const timestampCell = document.createElement('div')
		timestampCell.className = 'col-span-3 font-mono text-gray-400'
		timestampCell.textContent = new Date(data.timestamp).toLocaleString()
		row.appendChild(timestampCell)

		// 2. Source
		const sourceCell = document.createElement('div')
		sourceCell.className = 'col-span-2 font-mono text-cyan-400'
		sourceCell.textContent = data.source
		row.appendChild(sourceCell)

		// 3. Targets
		const targetsCell = document.createElement('div')
		targetsCell.className = 'col-span-2 font-mono text-purple-400'
		targetsCell.textContent = data.targets.join(', ')
		row.appendChild(targetsCell)

		// 4. Original Message
		const messageCell = document.createElement('div')
		messageCell.className = 'col-span-5 original-message overflow-x-auto'
		const pre = document.createElement('pre')
		pre.className = 'bg-gray-900/70 p-3 rounded-md text-xs'
		pre.textContent = JSON.stringify(data.original_message, null, 2)
		messageCell.appendChild(pre)
		row.appendChild(messageCell)

		logEntriesEl.appendChild(row)
	})

	// Auto-scroll to the bottom if no filters are active
	if (!sourceFilter && !targetsFilter && !messageFilter) {
		logEntriesEl.scrollTop = logEntriesEl.scrollHeight
	}
}

function connect() {
	if (
		!statusEl ||
		!logEntriesEl ||
		!sourceFilterEl ||
		!targetsFilterEl ||
		!messageFilterEl ||
		!maxLogInputEl ||
		!clearLogBtnEl
	) {
		console.error('Required elements not found in DOM')
		return
	}

	sourceFilterEl.addEventListener('input', renderLogs)
	targetsFilterEl.addEventListener('input', renderLogs)
	messageFilterEl.addEventListener('input', renderLogs)
	clearLogBtnEl.addEventListener('click', clearLogs)
	maxLogInputEl.addEventListener('change', () => {
		applyMaxLogLimit()
		renderLogs()
	})

	statusEl.textContent = `Connecting to ${SERVER_URL}...`
	const ws = new WebSocket(SERVER_URL)

	ws.onopen = () => {
		console.log('Connected to /ws/inspector')
		statusEl.textContent = 'Connected. Listening for messages...'
		statusEl.classList.remove('text-yellow-400', 'text-red-500')
		statusEl.classList.add('text-green-400')
	}

	ws.onmessage = event => {
		try {
			const data = JSON.parse(event.data)
			allLogs.push(data)
			applyMaxLogLimit()
			renderLogs()
		} catch (error) {
			console.error('Failed to parse incoming message:', error)
		}
	}

	ws.onclose = () => {
		console.log('Disconnected from /ws/inspector')
		statusEl.textContent = 'Disconnected. Retrying in 5 seconds...'
		statusEl.classList.remove('text-green-400', 'text-yellow-400')
		statusEl.classList.add('text-red-500')
		setTimeout(connect, 5000) // Retry connection after 5 seconds
	}

	ws.onerror = error => {
		console.error('WebSocket error:', error)
		statusEl.textContent = 'Connection error.'
		statusEl.classList.remove('text-green-400')
		statusEl.classList.add('text-red-500')
		ws.close()
	}
}

// Start the connection process
document.addEventListener('DOMContentLoaded', connect)
