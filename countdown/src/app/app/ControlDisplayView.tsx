import { type Control } from '@/types/control'

export function ControlDisplayView({
	control,
	onEditClick,
	onDelete,
}: {
	control: Control
	onEditClick: () => void
	onDelete: () => void
}) {
	return (
		<div className="border p-4 my-2 rounded-lg shadow-sm bg-white flex justify-between items-center">
			<div>
				<h2 className="font-bold text-lg">{control.name}</h2>
				<p className="text-sm text-gray-600 capitalize">{control.type}</p>
			</div>
			<div className="flex gap-2">
				<button onClick={onEditClick} className="px-4 py-2 rounded-md bg-gray-200 hover:bg-gray-300">
					Edit
				</button>
				<button onClick={onDelete} className="px-4 py-2 rounded-md bg-red-500 text-white hover:bg-red-600">
					Delete
				</button>
			</div>
		</div>
	)
}
