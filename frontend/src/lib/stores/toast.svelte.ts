interface Toast {
	id: number;
	message: string;
	type: 'success' | 'error' | 'info';
}

let toasts = $state<Toast[]>([]);
let nextId = 0;

export function getToasts() {
	return toasts;
}

export function addToast(message: string, type: Toast['type'], duration = 3000) {
	const id = nextId++;
	toasts = [...toasts, { id, message, type }];

	setTimeout(() => {
		toasts = toasts.filter((t) => t.id !== id);
	}, duration);
}

export const toast = {
	success: (message: string) => addToast(message, 'success'),
	error: (message: string) => addToast(message, 'error', 5000),
	info: (message: string) => addToast(message, 'info')
};
