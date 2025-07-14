import { type Control } from '@/types/control'
import { useState } from 'react'
import { NumberConstraintsEditor } from './NumberConstraintsEditor'
import { TextConstraintsEditor } from './TextConstraintsEditor'
import { SelectOptionsEditor } from './SelectOptionsEditor'

export function ControlEditForm({
	control,
	onSave,
	onCancel,
}: {
	control: Control
	onSave: (newControl: Control) => void
	onCancel: () => void
}) {
	const [editedControl, setEditedControl] = useState(control)

	const handleFieldChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
		const { name, value } = e.target
		setEditedControl(prev => {
			const newControl = { ...prev }
			if (name === 'name') {
				newControl.name = value
			} else if (newControl.type === 'number') {
				if (name === 'min' || name === 'max') {
					newControl[name] = parseInt(value, 10) || 0
				} else if (name === 'int') {
					newControl.int = (e.target as HTMLInputElement).checked
				}
			} else if (newControl.type === 'text' && name === 'regex') {
				newControl.regex = value
			}
			return newControl
		})
	}

	const handleTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		const newType = e.target.value
		let newControl: Control

		switch (newType) {
			case 'number':
				newControl = { name: editedControl.name, type: 'number', min: 0, max: 100, int: false }
				break
			case 'text':
				newControl = { name: editedControl.name, type: 'text', regex: '.*' }
				break
			case 'select':
				newControl = { name: editedControl.name, type: 'select', options: [] }
				break
			case 'button':
			default:
				newControl = { name: editedControl.name, type: 'button' }
				break
		}
		setEditedControl(newControl)
	}

	const handleOptionChange = (index: number, field: 'value' | 'label', newValue: string) => {
		setEditedControl(prev => {
			if (prev.type !== 'select' || !prev.options) return prev
			const newOptions = [...prev.options]
			newOptions[index] = { ...newOptions[index], [field]: newValue }
			return { ...prev, options: newOptions }
		})
	}

	const handleAddNewOption = () => {
		setEditedControl(prev => {
			if (prev.type !== 'select') return prev
			const newOptions = [...(prev.options || []), { value: '', label: '' }]
			return { ...prev, options: newOptions }
		})
	}

	const handleRemoveOption = (index: number) => {
		setEditedControl(prev => {
			if (prev.type !== 'select' || !prev.options) return prev
			const newOptions = prev.options.filter((_, i) => i !== index)
			return { ...prev, options: newOptions }
		})
	}

	return (
		<div className="border p-4 my-2 rounded-lg shadow-md bg-white">
			<div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
				<div>
					<label className="block text-sm font-medium text-gray-700">Name</label>
					<input
						type="text"
						name="name"
						value={editedControl.name}
						onChange={handleFieldChange}
						className="mt-1 block w-full px-3 py-2 bg-white border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
					/>
				</div>
				<div>
					<label className="block text-sm font-medium text-gray-700">Type</label>
					<select
						name="type"
						value={editedControl.type}
						onChange={handleTypeChange}
						className="mt-1 block w-full pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
					>
						<option value="button">Button</option>
						<option value="number">Number</option>
						<option value="text">Text</option>
						<option value="select">Select</option>
					</select>
				</div>
			</div>

			{editedControl.type === 'number' && (
				<NumberConstraintsEditor control={editedControl} onFieldChange={handleFieldChange} />
			)}

			{editedControl.type === 'text' && (
				<TextConstraintsEditor control={editedControl} onFieldChange={handleFieldChange} />
			)}

			{editedControl.type === 'select' && (
				<SelectOptionsEditor
					control={editedControl}
					onOptionChange={handleOptionChange}
					onAddNewOption={handleAddNewOption}
					onRemoveOption={handleRemoveOption}
				/>
			)}

			<div className="flex justify-end gap-2">
				<button onClick={onCancel} className="px-4 py-2 rounded-md border border-gray-300 hover:bg-gray-50">
					Cancel
				</button>
				<button
					onClick={() => onSave(editedControl)}
					className="px-4 py-2 rounded-md bg-blue-500 text-white hover:bg-blue-600"
				>
					Save
				</button>
			</div>
		</div>
	)
}
