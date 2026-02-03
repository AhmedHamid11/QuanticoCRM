export type TaskStatus = 'Open' | 'In Progress' | 'Completed' | 'Deferred' | 'Cancelled';
export type TaskPriority = 'Low' | 'Normal' | 'High' | 'Urgent';
export type TaskType = 'Call' | 'Email' | 'Meeting' | 'Todo';

export interface Task {
	id: string;
	orgId: string;
	subject: string;
	description: string;
	status: TaskStatus;
	priority: TaskPriority;
	type: TaskType;
	dueDate: string | null;
	parentId: string | null;
	parentType: string | null;
	parentName: string;
	assignedUserId: string | null;
	createdById: string | null;
	createdByName: string;
	modifiedById: string | null;
	modifiedByName: string;
	createdAt: string;
	modifiedAt: string;
	deleted: boolean;
	customFields?: Record<string, unknown>;
}

export interface TaskListResponse {
	data: Task[];
	total: number;
	page: number;
	pageSize: number;
	totalPages: number;
}

export interface TaskCreateInput {
	subject: string;
	description?: string;
	status?: TaskStatus;
	priority?: TaskPriority;
	type?: TaskType;
	dueDate?: string | null;
	parentId?: string | null;
	parentType?: string | null;
	parentName?: string;
	assignedUserId?: string | null;
	customFields?: Record<string, unknown>;
}

export interface TaskUpdateInput {
	subject?: string;
	description?: string;
	status?: TaskStatus;
	priority?: TaskPriority;
	type?: TaskType;
	dueDate?: string | null;
	parentId?: string | null;
	parentType?: string | null;
	parentName?: string;
	assignedUserId?: string | null;
	customFields?: Record<string, unknown>;
}

export const TASK_STATUSES: TaskStatus[] = ['Open', 'In Progress', 'Completed', 'Deferred', 'Cancelled'];
export const TASK_PRIORITIES: TaskPriority[] = ['Low', 'Normal', 'High', 'Urgent'];
export const TASK_TYPES: TaskType[] = ['Call', 'Email', 'Meeting', 'Todo'];
