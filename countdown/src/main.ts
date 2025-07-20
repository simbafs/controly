import './style.css'
import { Display } from 'controly-sdk'
import QRcode from 'qrcode'
import type { ErrorPayload } from 'controly-sdk/dist/types'

const SERVER_URL = import.meta.env.PROD ? 'wss://controly.1li.tw/ws' : 'ws://localhost:8080/ws'

const $ = document.querySelector.bind(document)

function initializeDisplay(serverUrl: string, { token, id }: { token?: string; id?: string } = {}) {
	const display = new Display({
		serverUrl: serverUrl,
		commandUrl: `${window.location}/command.json`,
		token,
		id,
	})

	display.on('open', id => {
		$<HTMLDivElement>('#app')!.innerHTML = `
      	<div id="qrcode" class="flex flex-col justify-center items-center bg-white p-10 rounded-2xl shadow-xl">
        	<!-- QR Code will be inserted here -->
      	</div>
      	<div id="time" class="text-[45vw] font-bold font-mono text-gray-900 hidden">60</div>
    	`
		const $qrcode = $<HTMLDivElement>('#qrcode')!
		console.log(id)
		if (!id) {
			console.error('Display ID is not available.')
			showErrorScreen('Display ID is not available.')
			return
		}

		QRcode.toDataURL(id, { width: 600 }).then(img => {
			$qrcode.innerHTML = `
      <img src="${img}" alt="Display QR Code" class="w-[300px] h-[300px]" />
      <p class="text-gray-600 text-2xl font-semibold mt-6 tracking-wider font-mono">${id}</p>
    `
		})
	})

	display.on('error', (error: ErrorPayload) => {
		console.error('Connection error:', error)
		showErrorScreen(`Connection failed: ${error.message} (code: ${error.code})`)
	})

	const handleSubscribe = ({ count }: { count: number }) => {
		const $qrcode = $('#qrcode') as HTMLDivElement
		const $time = $('#time') as HTMLDivElement
		console.log({ count })
		if (count === 0) {
			console.log('true')
			$qrcode.classList.remove('hidden')
			$time.classList.add('hidden')
		} else {
			console.log('false')
			$qrcode.classList.add('hidden')
			$time.classList.remove('hidden')
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

function showErrorScreen(message: string) {
	const app = document.querySelector<HTMLDivElement>('#app')!
	app.innerHTML = `
    <div class="w-full h-full flex flex-col justify-center items-center text-center">
      <div class="bg-white p-10 rounded-2xl shadow-xl flex flex-col gap-6 items-center w-full max-w-md">
        <h2 class="m-0 text-3xl font-bold text-red-600">An Error Occurred</h2>
        <p class="text-gray-600 text-lg">${message}</p>
        <button id="back-to-connect-btn" class="w-full p-3 text-lg font-semibold rounded-lg border-none bg-blue-600 text-white cursor-pointer transition-colors duration-200 hover:bg-blue-700" to Connection Page
        </button>
      </div>
    </div>
  `

	document.querySelector<HTMLButtonElement>('#back-to-connect-btn')!.addEventListener('click', () => {
		main()
	})
}

function main() {
	const app = document.querySelector<HTMLDivElement>('#app')!

	app.innerHTML = `
    <div class="w-full h-full flex flex-col justify-center items-center text-center">
      <div id="server-url-container" class="bg-white p-10 rounded-2xl shadow-xl flex flex-col gap-6 items-center w-full max-w-md">
        <h2 class="m-0 mb-2 text-gray-900">Enter Server URL</h2>
        <input type="text" id="server-url-input" placeholder="ws:/ws" value="${SERVER_URL}" class="w-full p-3 text-base border border-gray-300 rounded-lg text-center box-border" />
        <input type="password" id="token-input" placeholder="Enter token (optional)" class="w-full p-3 text-base border border-gray-300 rounded-lg text-center box-border" />
        <button id="connect-btn" class="w-full p-3 text-lg font-semibold rounded-lg border-none bg-blue-600 text-white cursor-pointer transition-colors duration-200 hover:bg-blue-700">Connect</button>
      </div>
    </div>
  `

	const connectBtn = $<HTMLButtonElement>('#connect-btn')!
	connectBtn.addEventListener('click', () => {
		const serverUrlInput = $<HTMLInputElement>('#server-url-input')!
		const tokenInput = $<HTMLInputElement>('#token-input')!
		const serverUrl = serverUrlInput.value.trim()
		const token = tokenInput.value.trim()

		if (!serverUrl) {
			alert('Please enter a server URL.')
			return
		}

		// Now initialize the display and the rest of the application
		initializeDisplay(serverUrl, {
			token,
		})
	})

	const param = new URLSearchParams(window.location.search)
	const serverUrl = param.get('serverUrl')
	const token = param.get('token')
	const id = param.get('id')

	if (serverUrl != null) {
		initializeDisplay(serverUrl, {
			token: token || undefined,
			id: id || undefined,
		})
	}
}

main()
