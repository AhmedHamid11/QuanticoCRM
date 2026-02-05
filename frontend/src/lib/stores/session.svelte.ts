// Session tracking for idle and absolute timeout
// Tracks user activity and enforces session limits

let idleTimer: ReturnType<typeof setTimeout> | null = null;
let absoluteTimer: ReturnType<typeof setTimeout> | null = null;
let onTimeoutCallback: (() => void) | null = null;

// Activity events to track
const ACTIVITY_EVENTS = ['mousedown', 'mousemove', 'keydown', 'scroll', 'touchstart', 'click'];

function resetIdleTimer(idleTimeoutMinutes: number) {
	if (idleTimer) {
		clearTimeout(idleTimer);
	}

	if (idleTimeoutMinutes > 0 && onTimeoutCallback) {
		idleTimer = setTimeout(() => {
			console.log('Session idle timeout reached');
			onTimeoutCallback?.();
		}, idleTimeoutMinutes * 60 * 1000);
	}
}

function handleActivity(idleTimeoutMinutes: number) {
	return () => resetIdleTimer(idleTimeoutMinutes);
}

let activityHandler: (() => void) | null = null;

export function initSessionTracking(
	idleTimeoutMinutes: number,
	absoluteTimeoutMinutes: number,
	onTimeout?: () => void
): void {
	if (typeof window === 'undefined') return;

	// Store callback for later use
	onTimeoutCallback = onTimeout || null;

	// Clean up any existing tracking
	stopSessionTracking();

	// Set up idle timeout tracking
	if (idleTimeoutMinutes > 0) {
		activityHandler = handleActivity(idleTimeoutMinutes);

		ACTIVITY_EVENTS.forEach(event => {
			window.addEventListener(event, activityHandler!, { passive: true });
		});

		// Start the idle timer
		resetIdleTimer(idleTimeoutMinutes);
	}

	// Set up absolute timeout
	if (absoluteTimeoutMinutes > 0 && onTimeoutCallback) {
		absoluteTimer = setTimeout(() => {
			console.log('Session absolute timeout reached');
			onTimeoutCallback?.();
		}, absoluteTimeoutMinutes * 60 * 1000);
	}
}

export function stopSessionTracking(): void {
	if (typeof window === 'undefined') return;

	// Clear timers
	if (idleTimer) {
		clearTimeout(idleTimer);
		idleTimer = null;
	}

	if (absoluteTimer) {
		clearTimeout(absoluteTimer);
		absoluteTimer = null;
	}

	// Remove activity listeners
	if (activityHandler) {
		ACTIVITY_EVENTS.forEach(event => {
			window.removeEventListener(event, activityHandler!);
		});
		activityHandler = null;
	}

	onTimeoutCallback = null;
}
