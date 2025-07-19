import './style.css'
import { Controller, type Command } from 'controly-sdk'
import { Html5QrcodeScanner } from 'html5-qrcode'

const $ = document.querySelector.bind(document)

const SERVER_URL = location.origin.replace('http', 'ws') + '/ws'

const controller = new Controller({ serverUrl: SERVER_URL })
let html5QrcodeScanner: Html5QrcodeScanner | null = null

function bindController(displayID: string, commandList: Command[], parent: HTMLDivElement) {
	const container = document.createElement('div')
	container.classList.add('control-group')
	container.dataset.displayId = displayID

	const title = document.createElement('h3')
	title.textContent = `Display: ${displayID}`
	container.appendChild(title)

	for (const cmd of commandList) {
		const controlWrapper = document.createElement('div')
		controlWrapper.classList.add('control')

		const label = document.createElement('label')
		if (cmd.type !== 'button' && cmd.label) {
			label.textContent = cmd.label
		}

		switch (cmd.type) {
			case 'text':
				const input = document.createElement('input')
				input.type = 'text'
				input.name = cmd.name
				input.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: input.value },
					})
				})
				controlWrapper.appendChild(label)
				controlWrapper.appendChild(input)
				break
			case 'number':
				const numberInput = document.createElement('input')
				numberInput.type = 'number'
				numberInput.name = cmd.name
				numberInput.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: parseFloat(numberInput.value) },
					})
				})
				if (cmd.default !== undefined) {
					numberInput.value = cmd.default.toString()
				}
				if (cmd.min !== undefined) {
					numberInput.min = cmd.min.toString()
				}
				if (cmd.max !== undefined) {
					numberInput.max = cmd.max.toString()
				}
				if (cmd.step !== undefined) {
					numberInput.step = cmd.step.toString()
				}
				controlWrapper.appendChild(label)
				controlWrapper.appendChild(numberInput)
				break
			case 'button':
				const btn = document.createElement('button')
				btn.textContent = cmd.label
				btn.name = cmd.name
				btn.addEventListener('click', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
					})
				})
				controlWrapper.appendChild(btn)
				break
			case 'select':
				const select = document.createElement('select')
				for (const option of cmd.options) {
					const opt = document.createElement('option')
					opt.value = option.value.toString()
					opt.textContent = option.label
					select.appendChild(opt)
				}
				select.name = cmd.name
				select.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: select.value },
					})
				})
				select.value = cmd.default?.toString() || ''
				controlWrapper.appendChild(label)
				controlWrapper.appendChild(select)
				break
			case 'checkbox':
				const checkbox = document.createElement('input')
				checkbox.type = 'checkbox'
				checkbox.name = cmd.name
				checkbox.checked = cmd.default || false
				checkbox.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: checkbox.checked },
					})
				})
				label.insertBefore(checkbox, label.firstChild)
				controlWrapper.appendChild(label)
				break
			default:
				console.warn(`Unknown command:`, cmd)
				continue // Skip unknown commands
		}
		container.appendChild(controlWrapper)
	}

	const status = document.createElement('div')
	status.id = `status-${displayID}`
	status.classList.add('status-display')
	container.appendChild(status)

	parent.appendChild(container)
}

function onScanSuccess(decodedText: string) {
	console.log(`Code matched = ${decodedText}`)
	const idInput = $<HTMLInputElement>('#id-input')!
	idInput.value = decodedText
	closeScanner()
	handleAddDisplay(decodedText)
}

function onScanFailure(error: any) {
	console.warn(`Code scan error = ${error}`)
}

function closeScanner() {
	if (html5QrcodeScanner) {
		html5QrcodeScanner.clear().catch(error => {
			console.error('Failed to clear html5QrcodeScanner.', error)
		})
		html5QrcodeScanner = null
		$<HTMLDivElement>('#scanner-container')!.style.display = 'none'
	}
}

function handleOpenScanner() {
	const scannerContainer = $<HTMLDivElement>('#scanner-container')!
	scannerContainer.style.display = 'flex'

	if (!html5QrcodeScanner) {
		html5QrcodeScanner = new Html5QrcodeScanner(
			'qr-reader',
			{ fps: 10, qrbox: { width: 250, height: 250 } },
			/* verbose= */ false,
		)
	}
	html5QrcodeScanner.render(onScanSuccess, onScanFailure)
}

function handleAddDisplay(id?: string) {
	const displayId = id || $<HTMLInputElement>('#id-input')!.value.trim()
	if (!displayId) return
	console.log(`Subscribing to display: ${displayId}`)
	controller.subscribe([displayId])
}

controller.on('open', () => {
	console.log('Controller connected')
	$<HTMLDivElement>('#connecting')!.style.display = 'none'
	$<HTMLDivElement>('#controller')!.style.display = 'flex'

	$<HTMLButtonElement>('#open-scanner')!.addEventListener('click', handleOpenScanner)
	$<HTMLButtonElement>('#connect-display')!.addEventListener('click', () => handleAddDisplay())
	$<HTMLButtonElement>('#close-scanner')!.addEventListener('click', closeScanner)
})

controller.on('close', () => {
	console.log('Controller disconnected')
	$<HTMLDivElement>('#connecting')!.style.display = 'block'
	$<HTMLDivElement>('#controller')!.style.display = 'none'

	$<HTMLButtonElement>('#open-scanner')!.removeEventListener('click', handleOpenScanner)
	$<HTMLButtonElement>('#connect-display')!.removeEventListener('click', () => handleAddDisplay())
	$<HTMLButtonElement>('#close-scanner')!.removeEventListener('click', closeScanner)
	closeScanner()
})

controller.on('command_list', (commandList, displayID) => {
	const existing = document.querySelector(`.control-group[data-display-id="${displayID}"]`)
	if (existing) {
		existing.remove()
	}
	bindController(displayID!, commandList, $<HTMLDivElement>('#controllers')!)
})

controller.on('status', (status, from) => {
	const statusDiv = $<HTMLDivElement>(`#status-${from}`)
	if (statusDiv) {
		statusDiv.innerHTML = ''
		for (const [key, value] of Object.entries(status)) {
			const p = document.createElement('p')
			p.textContent = `${key}: ${JSON.stringify(value)}`
			statusDiv.appendChild(p)
		}
	} else {
		console.warn(`No status div found for display ID: ${from}`)
	}
})

controller.on('display_disconnected', displayId => {
	const controlGroup = document.querySelector(`.control-group[data-display-id="${displayId}"]`)
	if (controlGroup) {
		controlGroup.remove()
		console.log(`Removed control group for disconnected display: ${displayId}`)
	}
})

controller.connect()

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <h1 id="connecting">Connecting...</h1>
  <div id="controller" style="display: none;">
    <div>
      <button id="open-scanner">Scan QR Code</button>
      <input type="text" id="id-input" placeholder="Enter Display ID" />
      <button id="connect-display">Connect</button>
    </div>
    <div id="controllers"></div>
  </div>

  <div id="scanner-container" style="display: none;">
    <div id="qr-reader"></div>
    <button id="close-scanner">Close Scanner</button>
  </div>
`
