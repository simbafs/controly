'use client'

import { App } from '@/types/app'
import { type Control } from '@/types/control'
import { useRef, useState } from 'react'
import { ControlComponent } from './ControlComponent'

export default function Page() {
	const [app, setApp] = useState<App | null>(null)

	const handleControlChange = (updatedControl: Control, index: number) => {
		if (!app) return
		app.controls[index] = updatedControl
		setApp({ ...app })
	}

	const handleControlDelete = (index: number) => {
		if (!app) return
		const newControls = app.controls.filter((_, i) => i !== index)
		setApp({ ...app, controls: newControls })
	}

	const handleAddControl = () => {
		if (!app) return
		const newControl: Control = {
			name: `new_control_${app.controls.length + 1}`,
			type: 'button',
		}
		setApp({ ...app, controls: [...app.controls, newControl] })
	}

	const handleSubmitChange = () => {
		// TODO:
		console.log(app)
	}

	if (!app) {
		return <ChooseApp setApp={setApp} />
	}

	return (
		<div className="container mx-auto p-4">
			<h1 className="text-2xl font-bold mb-4">{app.name}</h1>
			<div>
				{app.controls.map((c, index) => (
					<ControlComponent
						key={index}
						control={c}
						onChange={newControl => handleControlChange(newControl, index)}
						onDelete={() => handleControlDelete(index)}
					/>
				))}
			</div>
			<div className="flex justify-between">
				<button onClick={handleAddControl} className="mt-4 px-4 py-2 rounded-md bg-green-500 text-white">
					Add Control
				</button>
				<button onClick={handleSubmitChange} className="mt-4 px-4 py-2 rounded-md bg-blue-500 text-white">
					Submit Change
				</button>
			</div>
		</div>
	)
}

function useApps() {
	// TODO:
	return [
		{
			name: 'App 1',
			controls: [
				{
					type: 'button',
					name: 'start',
				},
				{
					type: 'number',
					name: 'n',
					min: 1,
					max: 5,
				},
				{
					type: 'text',
					name: 'message',
					regex: '^$',
				},
				{
					type: 'select',
					name: 'color',
					options: [
						{ value: '#ff0000', label: 'Red' },
						{ value: '#00ff00', label: 'Green' },
						{ value: '#0000ff', label: 'Blue' },
					],
				},
			],
		},
		{ name: 'App 2', controls: [] },
	] as App[]
}

function createApp(name: string, password: string): App {
	// TODO:
	console.log(`create app with name: ${name} and password: ${password}`)
	return {
		name,
		controls: [],
	}
}

function ChooseApp({ setApp }: { setApp: (app: App | null) => void }) {
	const apps = useApps()
	const nameRef = useRef<HTMLInputElement | null>(null)
	const passwordRef = useRef<HTMLInputElement | null>(null)

	return (
		<div className="w-1/2">
			<h1 className="text-xl font-semibold mb-2">Choose an existing app</h1>
			<div className="flex flex-col gap-2">
				{apps.map(app => (
					<button
						key={app.name}
						className="border border-gray-200 p-2 rounded-md text-left hover:bg-gray-50"
						onClick={() => setApp(app)}
					>
						{app.name}
					</button>
				))}
			</div>
			<h1 className="text-xl font-semibold mt-6 mb-2">Create new app</h1>
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
					className="border border-gray-300 p-2 rounded-md w-full mb-2"
					required
				/>
				<input
					type="password"
					placeholder="Password"
					ref={passwordRef}
					className="border border-gray-300 p-2 rounded-md w-full mb-2"
					required
				/>
				<button type="submit" className="bg-blue-500 text-white p-2 rounded-md w-full hover:bg-blue-600">
					Create
				</button>
			</form>
		</div>
	)
}
