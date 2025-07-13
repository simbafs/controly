import type { Metadata } from 'next'
import './globals.css'

export const metadata: Metadata = {
	title: 'Countdown Timer',
	description: 'An example of controly',
}

export default function RootLayout({
	children,
}: Readonly<{
	children: React.ReactNode
}>) {
	return (
		<html lang="en">
			<body className="flex flex-col h-screen w-screen py-8 items-center">{children}</body>
		</html>
	)
}
