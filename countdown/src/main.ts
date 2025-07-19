import './style.css'
import { Display } from 'controly-sdk'
import QRcode from 'qrcode'

const SERVER_URL = 'ws://localhost:8080/ws'

const $ = document.querySelector.bind(document)

const display = new Display({
	serverUrl: SERVER_URL,
	commandUrl: `${window.location}/command.json`,
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
		$qrcode.style.display = 'block'
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

document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
  <div id="qrcode"></div>
  <div id="time" style="display: none;">60</div>
`
