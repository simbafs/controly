import { TextControl } from '@/types/control'

export function TextConstraintsEditor({
	control,
	onFieldChange,
}: {
	control: TextControl
	onFieldChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}) {
	return (
		<div className="mb-4">
			<label className="block text-sm font-medium text-gray-700">Regex</label>
			<input
				type="text"
				name="regex"
				value={control.regex}
				onChange={onFieldChange}
				className="mt-1 block w-full px-3 py-2 bg-white border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm"
			/>
		</div>
	)
}
