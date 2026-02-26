export interface SchedulingPage {
	id: string;
	slug: string;
	title: string;
	description: string;
	durationMinutes: number;
	availability: WeeklyAvailability;
	timezone: string;
	isActive: boolean;
	bufferMinutes: number;
	maxDaysAhead: number;
	createdAt: string;
	updatedAt: string;
}

export interface WeeklyAvailability {
	monday?: TimeWindow[];
	tuesday?: TimeWindow[];
	wednesday?: TimeWindow[];
	thursday?: TimeWindow[];
	friday?: TimeWindow[];
	saturday?: TimeWindow[];
	sunday?: TimeWindow[];
}

export interface TimeWindow {
	start: string; // "09:00"
	end: string; // "17:00"
}

export interface SchedulingPagePublicView {
	slug: string;
	title: string;
	description: string;
	durationMinutes: number;
	availability: WeeklyAvailability;
	timezone: string;
	maxDaysAhead: number;
	ownerName: string;
}

export interface AvailableSlot {
	start: string; // ISO 8601
	end: string; // ISO 8601
}

export interface BookingInput {
	guestName: string;
	guestEmail: string;
	guestNotes?: string;
	startTime: string; // ISO 8601
}

export interface Booking {
	id: string;
	guestName: string;
	guestEmail: string;
	guestNotes: string;
	startTime: string;
	endTime: string;
	status: string;
	createdAt: string;
}

export type GoogleCalendarStatus = 'not_connected' | 'connected' | 'expired';

export const DAYS_OF_WEEK = [
	'monday',
	'tuesday',
	'wednesday',
	'thursday',
	'friday',
	'saturday',
	'sunday'
] as const;

export type DayOfWeek = (typeof DAYS_OF_WEEK)[number];

export const TIMEZONES = [
	'America/New_York',
	'America/Chicago',
	'America/Denver',
	'America/Los_Angeles',
	'America/Phoenix',
	'America/Anchorage',
	'Pacific/Honolulu',
	'Europe/London',
	'Europe/Paris',
	'Europe/Berlin',
	'Europe/Amsterdam',
	'Asia/Tokyo',
	'Asia/Shanghai',
	'Asia/Singapore',
	'Asia/Dubai',
	'Australia/Sydney',
	'UTC'
] as const;
