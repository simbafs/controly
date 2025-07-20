import './style.css'
import { Controller, type Command } from 'controly'
import { Html5QrcodeScanner } from 'html5-qrcode'

const $ = document.querySelector.bind(document)

const SERVER_URL = import.meta.env.PROD ? location.origin.replace('http', 'ws') + '/ws' : 'ws://localhost:8080/ws'

const controller = new Controller({ serverUrl: SERVER_URL })
let html5QrcodeScanner: Html5QrcodeScanner | null = null

// --- Tailwind CSS classes ---
const baseInputClass =
	'block w-full rounded-md border-0 px-2 py-2 text-gray-900 shadow-sm ring-1 ring-inset ring-gray-300 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-indigo-600 sm:text-sm sm:leading-6'
const baseButtonClass =
	'rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600'
const baseLabelClass = 'block text-sm font-medium leading-6 text-gray-900'
// ---

function bindController(displayID: string, commandList: Command[], parent: HTMLDivElement) {
	const container = document.createElement('div')
	container.className = 'w-full max-w-2xl rounded-xl border border-gray-200 bg-white p-6 shadow-sm'
	container.dataset.displayId = displayID

	const header = document.createElement('div')
	header.className = 'flex items-center justify-between border-b border-gray-200 pb-4 mb-4 -mt-2'

	const title = document.createElement('h3')
	title.className = 'text-lg font-semibold leading-7 text-gray-900'
	title.textContent = `Display: ${displayID}`

	const disconnectBtn = document.createElement('button')
	disconnectBtn.textContent = 'Disconnect'
	disconnectBtn.className =
		'rounded bg-red-600 px-2 py-1 text-xs font-semibold text-white shadow-sm hover:bg-red-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-600'
	disconnectBtn.addEventListener('click', () => {
		controller.unsubscribe([displayID])
		container.remove()
		console.log(`Unsubscribed from and removed display: ${displayID}`)
	})

	header.appendChild(title)
	header.appendChild(disconnectBtn)
	container.appendChild(header)

	const controlContainer = document.createElement('div')
	controlContainer.className = 'space-y-6 flex flex-col gap-1'

	for (const cmd of commandList) {
		switch (cmd.type) {
			case 'text':
				const input = document.createElement('input')
				input.type = 'text'
				input.name = cmd.name
				input.className = `${baseInputClass} mt-2`
				input.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: input.value },
					})
				})
				controlContainer.appendChild(input)
				break
			case 'number':
				const numberInput = document.createElement('input')
				numberInput.type = 'number'
				numberInput.name = cmd.name
				numberInput.className = `${baseInputClass} mt-2`
				numberInput.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: parseFloat(numberInput.value) },
					})
				})
				if (cmd.default !== undefined) numberInput.value = cmd.default.toString()
				if (cmd.min !== undefined) numberInput.min = cmd.min.toString()
				if (cmd.max !== undefined) numberInput.max = cmd.max.toString()
				if (cmd.step !== undefined) numberInput.step = cmd.step.toString()
				controlContainer.appendChild(numberInput)
				break
			case 'button':
				const btn = document.createElement('button')
				btn.textContent = cmd.label
				btn.name = cmd.name
				btn.className = baseButtonClass
				btn.addEventListener('click', () => {
					controller.sendCommand(displayID, { name: cmd.name })
				})
				controlContainer.appendChild(btn)
				break
			case 'select':
				const select = document.createElement('select')
				select.className = `${baseInputClass} mt-2`
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
				controlContainer.appendChild(select)
				break
			case 'checkbox':
				const checkboxWrapper = document.createElement('div')
				checkboxWrapper.className = 'flex items-center gap-x-3'
				const label = document.createElement('label')
				label.className = baseLabelClass
				const checkbox = document.createElement('input')
				checkbox.type = 'checkbox'
				checkbox.name = cmd.name
				checkbox.checked = cmd.default || false
				checkbox.className = 'h-4 w-4 rounded border-gray-300 text-indigo-600 focus:ring-indigo-600'
				checkbox.addEventListener('change', () => {
					controller.sendCommand(displayID, {
						name: cmd.name,
						args: { value: checkbox.checked },
					})
				})
				label.htmlFor = cmd.name
				checkbox.id = cmd.name
				checkboxWrapper.appendChild(checkbox)
				checkboxWrapper.appendChild(label)
				controlContainer.appendChild(checkboxWrapper)
				break
			default:
				console.warn(`Unknown command:`, cmd)
				continue // Skip unknown commands
		}
	}
	container.appendChild(controlContainer)

	const status = document.createElement('div')
	status.id = `status-${displayID}`
	status.className =
		'mt-6 pt-4 border-t border-dashed border-gray-200 font-mono text-xs break-all whitespace-pre-wrap text-gray-600'
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
	const $input = $<HTMLInputElement>('#id-input')!
	const displayId = id || $input!.value.trim()
	if (!displayId) return
	console.log(`Subscribing to display: ${displayId}`)
	controller.subscribe([displayId])
	$input.value = ''
}

controller.on('open', () => {
	console.log('Controller connected')
	$<HTMLDivElement>('#connecting')!.style.display = 'none'
	$<HTMLDivElement>('#controller')!.style.display = 'flex'

	$<HTMLButtonElement>('#open-scanner')!.addEventListener('click', handleOpenScanner)
	$<HTMLButtonElement>('#connect-display')!.addEventListener('click', () => handleAddDisplay())
	$<HTMLButtonElement>('#close-scanner')!.addEventListener('click', closeScanner)

	const param = new URLSearchParams(window.location.search)
	const ids = param.getAll('id')

	for (const id of ids) {
		console.log(id)
		handleAddDisplay(id)
	}
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
	const existing = document.querySelector(`[data-display-id="${displayID}"]`)
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
			p.className = 'm-2'
			p.textContent = `${key}: ${JSON.stringify(value)}`
			statusDiv.appendChild(p)
		}
	} else {
		console.warn(`No status div found for display ID: ${from}`)
	}
})

controller.on('display_disconnected', displayId => {
	const controlGroup = document.querySelector(`[data-display-id="${displayId}"]`)
	if (controlGroup) {
		controlGroup.remove()
		console.log(`Removed control group for disconnected display: ${displayId}`)
	}
})

controller.connect()

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <h1 id="connecting" class="text-2xl font-bold text-gray-500">Connecting...</h1>
  <div id="controller" class="hidden w-full max-w-md flex-col items-center gap-6">
    <div class="w-full">
      <button id="open-scanner" type="button" class="${baseButtonClass} w-full justify-center">Scan QR Code</button>
      
      <div class="relative my-4">
        <div class="absolute inset-0 flex items-center" aria-hidden="true">
          <div class="w-full border-t border-gray-300"></div>
        </div>
        <div class="relative flex justify-center">
          <span class="bg-gray-50 px-2 text-sm text-gray-500">or</span>
        </div>
      </div>

      <div class="flex gap-x-2">
        <input type="text" id="id-input" placeholder="Enter Display ID" class="${baseInputClass}" />
        <button id="connect-display" type="button" class="${baseButtonClass}">Connect</button>
      </div>
    </div>
    <div id="controllers" class="w-full space-y-6"></div>
  </div>

  <div id="scanner-container" class="fixed inset-0 z-50 hidden flex-col items-center justify-center gap-4 bg-black/80">
    <div id="qr-reader" class="w-[90%] max-w-lg overflow-hidden rounded-xl bg-white"></div>
    <button id="close-scanner" type="button" class="${baseButtonClass}">Close Scanner</button>
  </div>
`
