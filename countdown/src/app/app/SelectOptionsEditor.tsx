import { SelectControl } from '@/types/control'

export function SelectOptionsEditor({
	control,
	onOptionChange,
	onAddNewOption,
	onRemoveOption,
}: {
	control: SelectControl
	onOptionChange: (index: number, field: 'value' | 'label', newValue: string) => void
	onAddNewOption: () => void
	onRemoveOption: (index: number) => void
}) {
	return (
		<div className="mb-4">
			<label className="block text-sm font-medium text-gray-700">Options</label>
			<div className="mt-1 space-y-2">
				{(control.options || []).map((option, index) => (
					<div key={index} className="flex items-center gap-2">
						<input
							type="text"
							value={option.value}
							onChange={e => onOptionChange(index, 'value', e.target.value)}
							placeholder="Value"
							className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
						/>
						<input
							type="text"
							value={option.label}
							onChange={e => onOptionChange(index, 'label', e.target.value)}
							placeholder="Label"
							className="flex-1 px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
						/>
						<button
							type="button"
							onClick={() => onRemoveOption(index)}
							className="px-3 py-2 bg-red-500 text-white rounded-md hover:bg-red-600"
						>
							Remove
						</button>
					</div>
				))}
				<button
					type="button"
					onClick={onAddNewOption}
					className="px-4 py-2 bg-blue-500 text-white rounded-md hover:bg-blue-600"
				>
					Add New Option
				</button>
			</div>
		</div>
	)
}
