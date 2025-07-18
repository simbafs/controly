import './style.css'
import { setupQrCodeScanner } from './QrCodeScanner'
import { setupControlPanel } from './ControlPanel'
import { Controller } from 'controly-sdk'

const appElement = document.querySelector<HTMLDivElement>('#app')!
const SERVER_URL = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`;

let controlyController: Controller | null = null

const renderScanner = () => {
	appElement.innerHTML = `
    <div>
      <h1>Controly Controller</h1>
      <div id="qr-reader" style="width: 500px"></div>
      <div id="qr-status">Scan a QR code to connect to a display.</div>
    </div>
  `

	setupQrCodeScanner(document.querySelector<HTMLDivElement>('#qr-reader')!, async displayId => {
		const qrStatusElement = document.querySelector<HTMLDivElement>('#qr-status')!
		qrStatusElement.textContent = `Scanned Display ID: ${displayId}. Connecting to server...`

		if (controlyController) {
			controlyController.disconnect()
		}

		controlyController = new Controller({
			serverUrl: SERVER_URL,
			// Optionally, provide a unique ID for this controller instance
			// id: 'my-controller-' + Math.random().toString(36).substring(7),
		})

		controlyController.on('open', id => {
			console.log(`Controller connected with ID: ${id}`)
			qrStatusElement.textContent = `Controller connected with ID: ${id}. Subscribing to display ${displayId}...`
			controlyController!.subscribe([displayId])
		})

		controlyController.on('command_list', (payload, from) => {
			console.log(`Received command list from Display '${from}':`, payload)
			if (from === displayId) {
				appElement.innerHTML = '<div id="control-panel"></div>'
				setupControlPanel(
					document.querySelector<HTMLDivElement>('#control-panel')!,
					displayId,
					payload,
					controlyController!,
				)
			}
		})

		controlyController.on('status', data => {
			const { from, payload } = data
			console.log(`Status update from Display '${from}':`, payload)
			// Here you can update UI elements based on the status
		})

		controlyController.on('notification', notification => {
			console.info('Server notification:', notification.message)
			qrStatusElement.textContent = `Notification: ${notification.message}`
		})

		controlyController.on('error', error => {
			console.error('An error occurred:', error.message)
			qrStatusElement.textContent = `Error: ${error.message}. Please try again.`
			// Optionally, re-render scanner or provide retry option
			setTimeout(renderScanner, 3000) // Re-render scanner after 3 seconds
		})

		controlyController.on('close', () => {
			console.log('Connection closed.')
			qrStatusElement.textContent = 'Connection closed. Scanning again...'
			setTimeout(renderScanner, 1000) // Re-render scanner after 1 second
		})

		controlyController.connect()
	})
}

renderScanner()
