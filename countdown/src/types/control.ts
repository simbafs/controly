export type Control =
	| {
			type: 'button'
			name: string
	  }
	| {
			type: 'text'
			regex: string
	  }
	| {
			type: 'number'
			min: number
			max: number
			int: boolean
	  }
	| {
			type: 'select'
			options: {
				value: string
				text: string
			}[]
	  }
