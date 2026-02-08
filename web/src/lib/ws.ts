const DEFAULT_API_BASE = 'http://localhost:8000';

const apiBase =
	import.meta.env.VITE_API_URL ??
	(typeof window !== 'undefined' ? window.location.origin : DEFAULT_API_BASE);

export const WS_BASE = apiBase.replace(/^http/, 'ws');

type WSHandlers = {
	onMessage: (event: MessageEvent) => void;
	onOpen?: () => void;
	onClose?: (event: CloseEvent) => void;
	onError?: (event: Event) => void;
};

type WSOptions = {
	initialDelayMs?: number;
	maxDelayMs?: number;
	maxRetries?: number;
};

export function createReconnectingWebSocket(
	path: string,
	handlers: WSHandlers,
	options: WSOptions = {}
): { close: () => void } {
	const initialDelayMs = options.initialDelayMs ?? 500;
	const maxDelayMs = options.maxDelayMs ?? 5000;
	const maxRetries = options.maxRetries ?? Infinity;
	let socket: WebSocket | null = null;
	let closed = false;
	let retries = 0;
	let delay = initialDelayMs;

	const connect = () => {
		if (closed) return;
		socket = new WebSocket(`${WS_BASE}${path}`);
		socket.onopen = () => {
			retries = 0;
			delay = initialDelayMs;
			handlers.onOpen?.();
		};
		socket.onmessage = handlers.onMessage;
		socket.onerror = (event) => handlers.onError?.(event);
		socket.onclose = (event) => {
			handlers.onClose?.(event);
			if (closed || retries >= maxRetries) return;
			retries += 1;
			const nextDelay = Math.min(maxDelayMs, delay * 2);
			setTimeout(connect, delay);
			delay = nextDelay;
		};
	};

	connect();

	return {
		close: () => {
			closed = true;
			socket?.close();
		}
	};
}

export function createWebSocket(path: string): WebSocket {
	return new WebSocket(`${WS_BASE}${path}`);
}
