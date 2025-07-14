export type ButtonControl = {
	type: 'button'
	name: string
}

export type TextControl = {
	type: 'text'
	name: string
	regex: string
}

export type NumberControl = {
	type: 'number'
	name: string
	min: number
	max: number
	int: boolean
}

export type SelectControl = {
	type: 'select'
	name: string
	options: {
		value: string
		label: string
	}[]
}

export type Control = ButtonControl | TextControl | NumberControl | SelectControl
