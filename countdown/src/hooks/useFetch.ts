import { useEffect, useState } from 'react'

export function api(url: string, opt?: RequestInit) {
	return fetch(new URL(url, 'http://localhost:8080/'), opt)
}

export function useApi<T>(url: string) {
	const [data, setData] = useState<T | null>(null)

	useEffect(() => {
		api(url)
			.then(res => res.json())
			.then(setData)
			.catch(err => console.error(`Error fetching data from ${url}:`, err))
	}, [url])

	return data
}
