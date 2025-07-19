import './style.css'
import { Display } from 'controly-sdk'
import QRcode from 'qrcode'

const $ = document.querySelector.bind(document)

function initializeDisplay(serverUrl: string) {
	const display = new Display({
		serverUrl: serverUrl,
		commandUrl: `${window.location.origin}/command.json`,
	})

	display.on('open', id => {
		const $qrcode = $('#qrcode') as HTMLDivElement
		console.log(id)
		if (!id) {
			console.error('Display ID is not available.')
			return
		}

		QRcode.toDataURL(id).then(img => {
			$qrcode.innerHTML = `
      <img src="${img}" alt="Display QR Code" />
      <p class="display-id">${id}</p>
    `
		})
	})

	const handleSubscribe = ({ count }: { count: number }) => {
		const $qrcode = $('#qrcode') as HTMLDivElement
		const $time = $('#time') as HTMLDivElement
		console.log({ count })
		if (count === 0) {
			console.log('true')
			$qrcode.style.display = 'flex'
			$time.style.display = 'none'
		} else {
			console.log('false')
			$qrcode.style.display = 'none'
			$time.style.display = 'block'
		}
	}

	display.on('subscribed', handleSubscribe)
	display.on('unsubscribed', handleSubscribe)

	let timer: ReturnType<typeof setInterval> | null = null
	let init_time = 60
	let time = 60

	function updateTime(t: number) {
		time = t
		$('#time')!.textContent = `${time}`
		display.updateStatus({
			time,
		})
	}

	display.command('start', () => {
		if (timer !== null) {
			return
		}

		time--
		updateTime(time)

		timer = setInterval(() => {
			time--
			if (time <= 0) {
				clearInterval(timer!)
				timer = null
			}
			updateTime(time)
		}, 1000)
	})

	display.command('stop', () => {
		clearInterval(timer!)
		timer = null
	})

	display.command('reset', () => {
		updateTime(init_time)
	})

	display.command('set_time', ({ value }: { value: number }) => {
		clearInterval(timer!)
		timer = null
		time = init_time = value
		updateTime(time)
	})

	display.connect()
}

function main() {
	const app = document.querySelector<HTMLDivElement>('#app')!
	app.innerHTML = `
    <div id="server-url-container" style="display: flex; flex-direction: column; gap: 1rem; align-items: center;">
      <h2>Enter Server URL</h2>
      <input type="text" id="server-url-input" placeholder="ws://localhost:8080/ws" value="ws://localhost:8080/ws" />
      <button id="connect-btn">Connect</button>
    </div>
  `

	const connectBtn = $<HTMLButtonElement>('#connect-btn')!
	connectBtn.addEventListener('click', () => {
		const serverUrlInput = $<HTMLInputElement>('#server-url-input')!
		const serverUrl = serverUrlInput.value.trim()

		if (!serverUrl) {
			alert('Please enter a server URL.')
			return
		}

		// Clear the app container and set up the countdown view
		app.innerHTML = `
      <div id="qrcode"></div>
      <div id="time" style="display: none;">60</div>
    `

		// Now initialize the display and the rest of the application
		initializeDisplay(serverUrl)
	})
}

main()
