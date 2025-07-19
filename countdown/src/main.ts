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

		QRcode.toDataURL(id, { width: 600 }).then(img => {
			$qrcode.innerHTML = `
      <img src="${img}" alt="Display QR Code" class="w-[300px] h-[300px]" />
      <p class="text-gray-600 text-2xl font-semibold mt-6 tracking-wider font-mono">${id}</p>
    `
		})
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

function main() {
	const app = document.querySelector<HTMLDivElement>('#app')!
	app.innerHTML = `
    <div id="server-url-container" class="bg-white p-10 rounded-2xl shadow-xl flex flex-col gap-6 items-center w-full max-w-md">
      <h2 class="m-0 mb-2 text-gray-900">Enter Server URL</h2>
      <input type="text" id="server-url-input" placeholder="ws://localhost:8080/ws" value="ws://localhost:8080/ws" class="w-full p-3 text-base border border-gray-300 rounded-lg text-center box-border" />
      <button id="connect-btn" class="w-full p-3 text-lg font-semibold rounded-lg border-none bg-blue-600 text-white cursor-pointer transition-colors duration-200 hover:bg-blue-700">Connect</button>
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
      <div id="qrcode" class="flex flex-col justify-center items-center bg-white p-10 rounded-2xl shadow-xl">
        <!-- QR Code will be inserted here -->
      </div>
      <div id="time" class="text-[45vw] font-bold font-mono text-gray-900 hidden">60</div>
    `

		// Now initialize the display and the rest of the application
		initializeDisplay(serverUrl)
	})
}

main()
