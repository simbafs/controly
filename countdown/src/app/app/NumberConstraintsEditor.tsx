import { NumberControl } from '@/types/control'

export function NumberConstraintsEditor({
	control,
	onFieldChange,
}: {
	control: NumberControl
	onFieldChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}) {
	return (
		<div className="grid grid-cols-2 gap-4 mb-4">
			<div>
				<label className="block text-sm font-medium text-gray-700">Min</label>
				<input
					type="number"
					name="min"
					value={control.min}
					onChange={onFieldChange}
					className="mt-1 block w-full px-3 py-2 bg-white border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
				/>
			</div>
			<div>
				<label className="block text-sm font-medium text-gray-700">Max</label>
				<input
					type="number"
					name="max"
					value={control.max}
					onChange={onFieldChange}
					className="mt-1 block w-full px-3 py-2 bg-white border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
				/>
			</div>
		</div>
	)
}
