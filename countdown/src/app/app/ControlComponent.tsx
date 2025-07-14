import { useState } from 'react'
import { type Control } from '@/types/control'
import { ControlDisplayView } from './ControlDisplayView'
import { ControlEditForm } from './ControlEditForm'

export function ControlComponent({
	control,
	onChange,
	onDelete,
}: {
	control: Control
	onChange: (newControl: Control) => void
	onDelete: () => void
}) {
	const [isEditing, setIsEditing] = useState(false)

	return isEditing ? (
		<ControlEditForm
			control={control}
			onSave={newControl => {
				onChange(newControl)
				setIsEditing(false)
			}}
			onCancel={() => setIsEditing(false)}
		/>
	) : (
		<ControlDisplayView control={control} onEditClick={() => setIsEditing(true)} onDelete={onDelete} />
	)
}
