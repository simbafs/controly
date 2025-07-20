import './style.css'

const SERVER_URL = import.meta.env.PROD
	? location.origin.replace('http', 'ws') + '/ws/inspector'
	: 'ws://localhost:8080/ws/inspector'

const statusEl = document.getElementById('status')
const logEntriesEl = document.getElementById('log-entries')

function connect() {
	if (!statusEl || !logEntriesEl) {
		console.error('Required elements not found in DOM')
		return
	}

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

			// Auto-scroll to the bottom
			logEntriesEl.scrollTop = logEntriesEl.scrollHeight
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
