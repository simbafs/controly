'use client'

import { App } from '@/types/app'
import { useRef, useState } from 'react'

export default function Page() {
	const [app, setApp] = useState<App | null>(null)
	if (!app) {
		return <ChooseApp setApp={setApp} />
	}
	return <p>{app.name}</p>
}

function useApps() {
	// TODO:
	return [{ name: 'App 1' }, { name: 'App 2' }] as App[]
}

function createApp(name: string, password: string): App {
	// TODO:
	console.log(`create app with name: ${name} and password: ${password}`)
	return {
		name,
		controls: [],
	}
}

function ChooseApp({ setApp }: { setApp: (App: App | null) => void }) {
	const apps = useApps()
	const nameRef = useRef<HTMLInputElement | null>(null)
	const passwordRef = useRef<HTMLInputElement | null>(null)

	return (
		<div className="w-1/2">
			<h1>Choose exists</h1>
			<div className="flex flex-col gap-2">
				{apps.map(app => (
					<button className="border border-gray-200 p-2 rounded-md" onClick={() => setApp(app)}>
						{app.name}
					</button>
				))}
			</div>
			<h1>Create new</h1>
			<form
				onSubmit={e => {
					e.preventDefault()
					setApp(createApp(nameRef.current?.value || '', passwordRef.current?.value || ''))
				}}
			>
				<input
					type="text"
					placeholder="Name"
					ref={nameRef}
					className="border border-gray-200 p-2 rounded-md w-full mb-2"
					required
				/>
				<input
					type="password"
					placeholder="Password"
					ref={passwordRef}
					className="border border-gray-200 p-2 rounded-md w-full mb-2"
					required
				/>
				<button type="submit" className="bg-blue-500 text-white p-2 rounded-md w-full">
					Create
				</button>
			</form>
		</div>
	)
}
