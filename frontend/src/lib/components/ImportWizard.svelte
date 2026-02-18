<script lang="ts">
	import { PUBLIC_API_URL } from '$env/static/public';
	import { addToast } from '$lib/stores/toast.svelte';
	import { auth } from '$lib/stores/auth.svelte';

	const API_BASE = PUBLIC_API_URL || '/api/v1';

	interface Props {
		entityName: string;
		onComplete: () => void;
		onCancel: () => void;
	}

	let { entityName, onComplete, onCancel }: Props = $props();

	interface AvailableField {
		name: string;
		label: string;
		type: string;
		relatedEntity?: string; // For link fields
		relatedEntityFields?: AvailableField[]; // Fields of related entity for lookup matching
	}

	interface LookupResolution {
		matchField: string;
		createIfNotFound?: boolean;
		newRecordData?: Record<string, Record<string, any>>; // matchValue -> field values
	}

	type ImportMode = 'create' | 'update' | 'upsert' | 'delete';

	interface MissingLookup {
		fieldName: string;
		relatedEntity: string;
		matchValue: string;
		matchField: string;
		requiredFields: AvailableField[];
		rowIndices: number[];
	}

	interface AnalyzeLookupResponse {
		missingLookups: MissingLookup[];
	}

	interface PreviewResponse {
		headers: string[];
		mappedHeaders: string[];
		sampleRows: Record<string, any>[];
		totalRows: number;
		unmappedColumns: string[];
		fields: FieldMapping[];
		availableFields: AvailableField[];
	}

	interface FieldMapping {
		csvHeader: string;
		fieldName: string;
		fieldLabel: string;
		fieldType: string;
		mapped: boolean;
	}

	interface ValidationIssue {
		row: number;
		column: string;
		fieldName: string;
		value: string;
		issueType: string;
		message: string;
		expected?: string;
	}

	interface AnalyzeResult {
		valid: boolean;
		totalRows: number;
		validRows: number;
		invalidRows: number;
		issues: ValidationIssue[];
		mappedFields: string[];
		missingRequired: string[];
	}

	interface DedupDecisionInput {
		keptExternalId: string;
		discardedExternalId: string;
		matchField: string;
		matchValue: string;
		decisionType: string;
		action: string;
		matchedRecordId?: string;
	}

	interface ImportResponse {
		created: number;
		updated: number;
		deleted?: number;
		failed: number;
		skipped?: number;
		merged?: number;
		totalRows: number;
		errors?: Array<{ index: number; error: string }>;
		ids?: string[];
		auditReport?: string; // Base64-encoded CSV
		importId?: string; // ID of the persisted import job
		warnings?: string[]; // Non-fatal warnings (e.g. dedup persistence failure)
	}

	interface ImportMatchCandidate {
		recordId: string;
		name: string;
		score: number;
	}

	interface ImportDuplicateMatch {
		importRowIndex: number;
		importRow: Record<string, any>;
		matchedRecordId: string;
		matchedRecord: Record<string, any>;
		confidenceScore: number;
		confidenceTier: string; // "high", "medium", "low"
		matchedFields: string[];
		ruleName: string;
		otherMatches?: ImportMatchCandidate[];
	}

	interface ImportDuplicateGroup {
		groupId: string;
		rowIndices: number[];
		rows: Record<string, any>[];
		keepIndex: number;
	}

	interface DuplicateCheckResult {
		databaseMatches: ImportDuplicateMatch[];
		withinFileGroups: ImportDuplicateGroup[];
		totalRows: number;
		flaggedRows: number;
	}

	interface ImportResolution {
		action: 'skip' | 'update' | 'import' | 'merge';
		selectedMatchId?: string;
	}

	let step = $state(1); // 1=Upload, 2=Validate, 2.5=CreateLookups, 2.75=DuplicateReview, 3=Import
	let file: File | null = $state(null);
	let previewData: PreviewResponse | null = $state(null);
	let columnMapping: Record<string, string> = $state({});
	let lookupResolution: Record<string, LookupResolution> = $state({}); // fieldName -> resolution config
	let missingLookups: MissingLookup[] = $state([]); // Lookups that need to be created
	let newRecordData: Record<string, Record<string, Record<string, any>>> = $state({}); // fieldName -> matchValue -> field values
	let analyzeResult: AnalyzeResult | null = $state(null);
	let importResult: ImportResponse | null = $state(null);
	let loading = $state(false);
	let loadingMessage = $state('');
	let error = $state('');
	let availableFields: AvailableField[] = $state([]);

	// Import mode settings
	let importMode: ImportMode = $state('create');
	let matchField: string = $state(''); // For update/upsert/delete - which field identifies records

	// Duplicate detection state
	let duplicateResult: DuplicateCheckResult | null = $state(null);
	let resolutions: Map<number, ImportResolution> = $state(new Map());
	let withinFileSelections: Map<string, number> = $state(new Map()); // groupId -> keepIndex
	let checkingDuplicates = $state(false);
	let duplicateCheckError = $state('');
	let showAllClear = $state(false);

	// Audit report field picker state
	let auditHeaders: string[] = $state([]);
	let auditRows: string[][] = $state([]);
	let selectedAuditFields: Set<string> = $state(new Set());

	const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB - must match backend UploadBodyLimit

	// Drag and drop state
	let isDragging = $state(false);
	let fileInputRef: HTMLInputElement | undefined = $state(undefined);

	// Handle file selection from input
	function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		if (!input.files || input.files.length === 0) return;
		processFile(input.files[0]);
	}

	// Handle drop
	function handleDrop(event: DragEvent) {
		event.preventDefault();
		isDragging = false;
		const files = event.dataTransfer?.files;
		if (!files || files.length === 0) return;
		const dropped = files[0];
		if (!dropped.name.toLowerCase().endsWith('.csv')) {
			error = 'Please drop a CSV file.';
			return;
		}
		processFile(dropped);
	}

	function handleDragOver(event: DragEvent) {
		event.preventDefault();
		isDragging = true;
	}

	function handleDragLeave(event: DragEvent) {
		event.preventDefault();
		isDragging = false;
	}

	// Process selected/dropped file
	async function processFile(selected: File) {
		if (selected.size > MAX_FILE_SIZE) {
			const sizeMB = (selected.size / (1024 * 1024)).toFixed(1);
			error = `File is too large (${sizeMB} MB). Maximum allowed size is 10 MB. Please split your file into smaller chunks and import them separately.`;
			return;
		}

		file = selected;
		error = '';
		loading = true;

		try {
			// Upload file for preview
			const formData = new FormData();
			formData.append('file', file);

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/preview`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Failed to preview CSV');
			}

			previewData = await response.json();
			const preview = previewData!;

			// Initialize column mapping with auto-mapped fields
			const mapping: Record<string, string> = {};
			preview.headers.forEach((header: string, idx: number) => {
				if (preview.mappedHeaders[idx]) {
					// Skip auto-mapping to 'id' field in insert mode (ID is auto-generated)
					if (importMode === 'create' && preview.mappedHeaders[idx].toLowerCase() === 'id') return;
					mapping[header] = preview.mappedHeaders[idx];
				}
			});
			columnMapping = mapping;

			// Get available fields for dropdowns (use the full list from backend)
			availableFields = preview.availableFields || [];
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load file';
			file = null;
		} finally {
			loading = false;
		}
	}

	// Proceed to validation
	async function analyzeData() {
		if (!file) return;

		loading = true;
		error = '';

		try {
			// For delete mode, auto-map the match field column (no manual column mapping needed)
			if (importMode === 'delete' && matchField && previewData?.headers) {
				const matchHeader = previewData.headers.find((h: string) => h.toLowerCase() === matchField.toLowerCase());
				if (matchHeader) {
					columnMapping = { [matchHeader]: matchField };
				}
			}
			// Check if any lookups have createIfNotFound enabled
			const hasCreateIfNotFound = Object.values(lookupResolution).some(r => r.createIfNotFound);

			// If createIfNotFound is enabled, first check for missing lookups
			if (hasCreateIfNotFound) {
				const lookupFormData = new FormData();
				lookupFormData.append('file', file);
				lookupFormData.append('options', JSON.stringify({ columnMapping, lookupResolution }));

				const lookupResponse = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze-lookups`, {
					method: 'POST',
					body: lookupFormData,
					credentials: 'include',
					headers: {
						'Authorization': `Bearer ${auth.accessToken || ''}`,
						'X-CSRF-Token': getCsrfToken()
					}
				});

				if (lookupResponse.ok) {
					const lookupResult: AnalyzeLookupResponse = await lookupResponse.json();
					if (lookupResult.missingLookups && lookupResult.missingLookups.length > 0) {
						missingLookups = lookupResult.missingLookups;
						// Initialize newRecordData for each missing lookup
						for (const lookup of missingLookups) {
							if (!newRecordData[lookup.fieldName]) {
								newRecordData[lookup.fieldName] = {};
							}
							if (!newRecordData[lookup.fieldName][lookup.matchValue]) {
								newRecordData[lookup.fieldName][lookup.matchValue] = {};
							}
						}
						step = 2.5; // Show create lookups step
						loading = false;
						return;
					}
				}
			}

			// Standard validation
			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify({ columnMapping, mode: importMode }));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Validation failed');
			}

			analyzeResult = await response.json();

			// For delete mode, skip Step 2 entirely — no duplicate check needed
			if (importMode === 'delete') {
				await executeImport();
				return;
			}

			step = 2;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Validation failed';
		} finally {
			loading = false;
		}
	}

	// Execute import
	async function executeImport() {
		if (!file) return;

		loading = true;
		error = '';
		const totalRows = analyzeResult?.totalRows || previewData?.totalRows || 0;
		const modeLabel = importMode === 'delete' ? 'Deleting' : importMode === 'update' ? 'Updating' : importMode === 'upsert' ? 'Upserting' : 'Importing';
		loadingMessage = `${modeLabel} ${totalRows.toLocaleString()} records...`;

		try {
			// For delete mode, ensure match field column is mapped
			if (importMode === 'delete' && matchField && previewData?.headers) {
				const matchHeader = previewData.headers.find((h: string) => h.toLowerCase() === matchField.toLowerCase());
				if (matchHeader) {
					columnMapping = { [matchHeader]: matchField };
				}
			}

			// Build final lookup resolution with newRecordData
			const finalLookupResolution: Record<string, LookupResolution> = {};
			for (const [fieldName, resolution] of Object.entries(lookupResolution)) {
				finalLookupResolution[fieldName] = {
					...resolution,
					newRecordData: newRecordData[fieldName] || undefined
				};
			}

			// Auto-detect external ID field from column mapping (look for 'Id' or 'SF_ID' columns)
			let detectedExternalIdField = '';
			for (const [csvHeader, fieldName] of Object.entries(columnMapping)) {
				const headerLower = csvHeader.toLowerCase();
				if (headerLower === 'id' || headerLower === 'sf_id' || headerLower === 'salesforce_id' || headerLower === 'sfid' || headerLower === 'external_id') {
					detectedExternalIdField = csvHeader;
					break;
				}
			}

			// Build options object
			const options: any = {
				columnMapping,
				lookupResolution: Object.keys(finalLookupResolution).length > 0 ? finalLookupResolution : undefined,
				mode: importMode,
				matchField: (importMode !== 'create' && matchField) ? matchField : undefined,
				externalIdField: detectedExternalIdField || undefined,
				skipErrors: false
			};

			// Add duplicate resolutions if any were made
			if (resolutions.size > 0) {
				const resolutionObj: Record<number, ImportResolution> = {};
				resolutions.forEach((value, key) => {
					resolutionObj[key] = value;
				});
				options.duplicateResolutions = resolutionObj;
			}

			// Add within-file skip indices and build dedup decisions
			const dedupDecisions: DedupDecisionInput[] = [];
			// Column containing the Salesforce external ID (e.g. "Id", "SF_ID")
			const externalIdCol = options.externalIdField || 'Id';

			if (withinFileSelections.size > 0 && duplicateResult?.withinFileGroups) {
				const skipIndices: number[] = [];
				for (const group of duplicateResult.withinFileGroups) {
					const keepIdx = withinFileSelections.get(group.groupId) ?? group.keepIndex;
					// -1 means "Keep All" — don't skip any rows in this group
					if (keepIdx === -1) continue;
					for (const rowIdx of group.rowIndices) {
						if (rowIdx !== keepIdx) {
							skipIndices.push(rowIdx);
							// Build dedup decision for within-file duplicates
							const keptRow = group.rows[group.rowIndices.indexOf(keepIdx)];
							const discardedRow = group.rows[group.rowIndices.indexOf(rowIdx)];
							if (keptRow && discardedRow) {
								// groupId format: "matchField:matchValue"
								const parts = group.groupId.split(':');
								dedupDecisions.push({
									keptExternalId: String(keptRow[externalIdCol] || ''),
									discardedExternalId: String(discardedRow[externalIdCol] || ''),
									matchField: parts[0] || '',
									matchValue: parts.slice(1).join(':') || '',
									decisionType: 'within_file',
									action: 'skip'
								});
							}
						}
					}
				}
				if (skipIndices.length > 0) {
					options.withinFileSkipIndices = skipIndices;
				}
			}

			// Build dedup decisions from database match resolutions
			if (resolutions.size > 0 && duplicateResult?.databaseMatches) {
				resolutions.forEach((resolution, rowIdx) => {
					const match = duplicateResult!.databaseMatches.find(m => m.importRowIndex === rowIdx);
					if (match) {
						dedupDecisions.push({
							keptExternalId: resolution.action === 'skip' ? '' : String(match.importRow[externalIdCol] || ''),
							discardedExternalId: resolution.action === 'skip' ? String(match.importRow[externalIdCol] || '') : '',
							matchField: (match.matchedFields && match.matchedFields[0]) || '',
							matchValue: match.matchedFields ? String(match.importRow[match.matchedFields[0]] || '') : '',
							decisionType: 'db_match',
							action: resolution.action,
							matchedRecordId: match.matchedRecordId || resolution.selectedMatchId || ''
						});
					}
				});
			}

			if (dedupDecisions.length > 0) {
				options.dedupDecisions = dedupDecisions;
			}

			const formData = new FormData();
			formData.append('file', file);
			formData.append('options', JSON.stringify(options));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Import failed');
			}

			importResult = await response.json();
			if (importResult?.auditReport) {
				parseAuditCSV(importResult.auditReport);
			}
			step = 3;
			// Build success message based on mode
			const result = importResult!;
			const messages: string[] = [];
			if (result.created > 0) messages.push(`${result.created} created`);
			if (result.updated > 0) messages.push(`${result.updated} updated`);
			if (result.deleted && result.deleted > 0) messages.push(`${result.deleted} deleted`);
			if (result.skipped && result.skipped > 0) messages.push(`${result.skipped} skipped`);
			if (result.merged && result.merged > 0) messages.push(`${result.merged} sent to merge`);
			addToast(`Import complete: ${messages.join(', ') || 'No changes'}`, 'success');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import failed';
			addToast('Import failed', 'error');
		} finally {
			loading = false;
			loadingMessage = '';
		}
	}

	function getCsrfToken(): string {
		if (typeof document === 'undefined') return '';
		const match = document.cookie.match(/(?:^|; )csrf_token=([^;]*)/);
		return match ? decodeURIComponent(match[1]) : '';
	}

	function goBack() {
		if (step === 2) {
			step = 1;
			analyzeResult = null;
		} else if (step === 2.5) {
			step = 1;
			missingLookups = [];
		}
	}

	// Proceed from create lookups step to validation
	async function proceedFromLookups() {
		loading = true;
		error = '';

		try {
			const formData = new FormData();
			formData.append('file', file!);
			formData.append('options', JSON.stringify({ columnMapping, mode: importMode }));

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/analyze`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Validation failed');
			}

			analyzeResult = await response.json();
			step = 2;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Validation failed';
		} finally {
			loading = false;
		}
	}

	function updateMapping(csvHeader: string, fieldName: string) {
		columnMapping[csvHeader] = fieldName;
		// Clear lookup resolution if field changed
		const baseFieldName = fieldName.replace(/Id$/, '');
		if (!fieldName || !isLinkField(baseFieldName)) {
			delete lookupResolution[baseFieldName];
		}
	}

	function isLinkField(fieldName: string): boolean {
		const field = availableFields.find(f => f.name === fieldName);
		return field?.type === 'link';
	}

	function getFieldForMapping(mappedFieldName: string): AvailableField | undefined {
		// mappedFieldName might be "accountId" or "account"
		const baseFieldName = mappedFieldName.replace(/Id$/, '');
		return availableFields.find(f => f.name === baseFieldName || f.name === mappedFieldName);
	}

	function enableLookup(fieldName: string, lookupField: string) {
		if (lookupField) {
			lookupResolution[fieldName] = {
				matchField: lookupField,
				createIfNotFound: lookupResolution[fieldName]?.createIfNotFound || false
			};
		} else {
			delete lookupResolution[fieldName];
		}
		lookupResolution = { ...lookupResolution };
	}

	function setLookupNotFoundAction(fieldName: string, action: 'fail' | 'create') {
		if (lookupResolution[fieldName]) {
			lookupResolution[fieldName] = {
				...lookupResolution[fieldName],
				createIfNotFound: action === 'create'
			};
			lookupResolution = { ...lookupResolution };
		}
	}

	function updateNewRecordField(fieldName: string, matchValue: string, field: string, value: string) {
		if (!newRecordData[fieldName]) {
			newRecordData[fieldName] = {};
		}
		if (!newRecordData[fieldName][matchValue]) {
			newRecordData[fieldName][matchValue] = {};
		}
		newRecordData[fieldName][matchValue][field] = value;
		newRecordData = { ...newRecordData };
	}

	// Check if all required fields are filled for missing lookups
	function canProceedFromLookups(): boolean {
		for (const lookup of missingLookups) {
			if (lookup.requiredFields) {
				for (const reqField of lookup.requiredFields) {
					const value = newRecordData[lookup.fieldName]?.[lookup.matchValue]?.[reqField.name];
					if (!value || value.trim() === '') {
						return false;
					}
				}
			}
		}
		return true;
	}

	// Duplicate check progress state
	let dupCheckPhase = $state('');
	let dupCheckProcessed = $state(0);
	let dupCheckTotal = $state(0);
	let dupCheckDuplicates = $state(0);

	// Duplicate check and resolution functions
	async function checkDuplicates() {
		if (!file) return;

		checkingDuplicates = true;
		duplicateCheckError = '';
		duplicateResult = null;
		showAllClear = false;
		dupCheckPhase = '';
		dupCheckProcessed = 0;
		dupCheckTotal = 0;
		dupCheckDuplicates = 0;

		try {
			const formData = new FormData();
			formData.append('file', file);

			// Include column mapping if set
			if (Object.keys(columnMapping).length > 0) {
				formData.append('options', JSON.stringify({ columnMapping }));
			}

			const response = await fetch(`${API_BASE}/entities/${entityName}/import/csv/check-duplicates`, {
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers: {
					'Authorization': `Bearer ${auth.accessToken || ''}`,
					'X-CSRF-Token': getCsrfToken()
				}
			});

			if (!response.ok) {
				const errorData = await response.json();
				throw new Error(errorData.error || 'Duplicate check failed');
			}

			// Read NDJSON stream
			const reader = response.body!.getReader();
			const decoder = new TextDecoder();
			let buffer = '';

			while (true) {
				const { done, value } = await reader.read();
				if (done) break;

				buffer += decoder.decode(value, { stream: true });
				const lines = buffer.split('\n');
				// Keep the last incomplete line in the buffer
				buffer = lines.pop() || '';

				for (const line of lines) {
					if (!line.trim()) continue;
					const event = JSON.parse(line);

					if (event.type === 'progress') {
						dupCheckPhase = event.phase;
						dupCheckProcessed = event.processedRows;
						dupCheckTotal = event.totalRows;
						dupCheckDuplicates = event.duplicatesFound;
					} else if (event.type === 'error') {
						throw new Error(event.error);
					} else if (event.type === 'result') {
						duplicateResult = event.result;
					}
				}
			}

			// Process any remaining buffer
			if (buffer.trim()) {
				const event = JSON.parse(buffer);
				if (event.type === 'result') {
					duplicateResult = event.result;
				} else if (event.type === 'error') {
					throw new Error(event.error);
				}
			}

			if (!duplicateResult) {
				throw new Error('No result received from duplicate check');
			}

			const hasDbMatches = duplicateResult!.databaseMatches && duplicateResult!.databaseMatches.length > 0;
			const hasFileGroups = duplicateResult!.withinFileGroups && duplicateResult!.withinFileGroups.length > 0;

			if (hasDbMatches || hasFileGroups) {
				// Initialize default resolutions based on confidence
				resolutions = new Map();
				for (const match of (duplicateResult!.databaseMatches || [])) {
					const defaultAction = match.confidenceScore >= 0.95 ? 'skip' : 'import';
					resolutions.set(match.importRowIndex, {
						action: defaultAction,
						selectedMatchId: match.matchedRecordId,
					});
				}

				// Initialize within-file selections (default to first row in each group)
				withinFileSelections = new Map();
				for (const group of (duplicateResult!.withinFileGroups || [])) {
					withinFileSelections.set(group.groupId, group.keepIndex);
				}

				step = 2.75; // Show review step
			} else {
				// No duplicates - show brief "all clear" and proceed to import
				showAllClear = true;
				setTimeout(() => {
					showAllClear = false;
					executeImport();
				}, 2000);
			}
		} catch (err) {
			duplicateCheckError = err instanceof Error ? err.message : 'Failed to check duplicates';
		} finally {
			checkingDuplicates = false;
		}
	}

	function setResolution(rowIndex: number, action: ImportResolution['action'], matchId?: string) {
		const newMap = new Map(resolutions);
		newMap.set(rowIndex, { action, selectedMatchId: matchId });
		resolutions = newMap;
	}

	function setWithinFileSelection(groupId: string, keepIndex: number) {
		const newMap = new Map(withinFileSelections);
		newMap.set(groupId, keepIndex);
		withinFileSelections = newMap;
	}

	function bulkResolve(action: 'skip' | 'import') {
		const newMap = new Map(resolutions);
		for (const match of (duplicateResult?.databaseMatches || [])) {
			// Only set resolution if not already resolved
			if (!newMap.has(match.importRowIndex)) {
				newMap.set(match.importRowIndex, {
					action,
					selectedMatchId: match.matchedRecordId,
				});
			}
		}
		resolutions = newMap;
	}

	function getResolvedCount(): number {
		return resolutions.size + withinFileSelections.size;
	}

	function getTotalFlaggedCount(): number {
		return (duplicateResult?.databaseMatches?.length || 0) + (duplicateResult?.withinFileGroups?.length || 0);
	}

	function allResolved(): boolean {
		const dbMatchCount = duplicateResult?.databaseMatches?.length || 0;
		const fileGroupCount = duplicateResult?.withinFileGroups?.length || 0;

		// All database matches must have resolutions
		const allDbResolved = resolutions.size >= dbMatchCount;

		// All within-file groups must have selections
		const allFileResolved = withinFileSelections.size >= fileGroupCount;

		return allDbResolved && allFileResolved;
	}

	function proceedToImport() {
		if (!allResolved()) return;
		executeImport();
	}

	function getConfidenceColor(tier: string): string {
		switch (tier) {
			case 'high': return 'text-red-600 bg-red-50';
			case 'medium': return 'text-yellow-600 bg-yellow-50';
			case 'low': return 'text-blue-600 bg-blue-50';
			default: return 'text-gray-600 bg-gray-50';
		}
	}

	function skipDuplicateCheck() {
		// Allow user to skip duplicate check if it failed
		duplicateCheckError = '';
		executeImport();
	}

	function parseCSVLine(line: string): string[] {
		const fields: string[] = [];
		let current = '';
		let inQuotes = false;
		for (let i = 0; i < line.length; i++) {
			const ch = line[i];
			if (inQuotes) {
				if (ch === '"') {
					if (i + 1 < line.length && line[i + 1] === '"') {
						current += '"';
						i++;
					} else {
						inQuotes = false;
					}
				} else {
					current += ch;
				}
			} else {
				if (ch === '"') {
					inQuotes = true;
				} else if (ch === ',') {
					fields.push(current);
					current = '';
				} else {
					current += ch;
				}
			}
		}
		fields.push(current);
		return fields;
	}

	function parseAuditCSV(base64Data: string) {
		const binaryString = atob(base64Data);
		const bytes = new Uint8Array(binaryString.length);
		for (let i = 0; i < binaryString.length; i++) {
			bytes[i] = binaryString.charCodeAt(i);
		}
		const decoder = new TextDecoder('utf-8');
		const csvText = decoder.decode(bytes);
		const lines = csvText.split('\n').filter(l => l.trim().length > 0);
		if (lines.length === 0) return;

		auditHeaders = parseCSVLine(lines[0]);
		auditRows = [];
		for (let i = 1; i < lines.length; i++) {
			auditRows.push(parseCSVLine(lines[i]));
		}
		// Dynamic data columns are everything after the 5 fixed columns
		const dataFields = auditHeaders.slice(5);
		selectedAuditFields = new Set(dataFields);
	}

	function csvEscape(value: string): string {
		if (value.includes(',') || value.includes('"') || value.includes('\n')) {
			return '"' + value.replace(/"/g, '""') + '"';
		}
		return value;
	}

	function downloadFilteredAuditReport() {
		const fixedCount = 5;
		const includeIndices: number[] = [];
		for (let i = 0; i < fixedCount && i < auditHeaders.length; i++) {
			includeIndices.push(i);
		}
		for (let i = fixedCount; i < auditHeaders.length; i++) {
			if (selectedAuditFields.has(auditHeaders[i])) {
				includeIndices.push(i);
			}
		}

		const filteredHeaders = includeIndices.map(i => auditHeaders[i]);
		const lines = [filteredHeaders.map(h => csvEscape(h)).join(',')];
		for (const row of auditRows) {
			const filteredRow = includeIndices.map(i => csvEscape(row[i] || ''));
			lines.push(filteredRow.join(','));
		}

		const csvContent = lines.join('\n');
		const blob = new Blob([csvContent], { type: 'text/csv' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `import-audit-${entityName}-${new Date().toISOString().split('T')[0]}.csv`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);
	}
</script>

<div class="max-w-6xl mx-auto">
	<!-- Step Indicator -->
	<div class="flex items-center justify-center mb-8">
		<div class="flex items-center">
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 1}
				class:bg-gray-300={step < 1}
				class:text-white={step >= 1}
			>
				1
			</div>
			<div class="w-24 h-1 mx-2" class:bg-blue-600={step >= 2} class:bg-gray-300={step < 2}></div>
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 2}
				class:bg-gray-300={step < 2}
				class:text-white={step >= 2}
			>
				2
			</div>
			<div class="w-24 h-1 mx-2" class:bg-blue-600={step >= 3} class:bg-gray-300={step < 3}></div>
			<div class="flex items-center justify-center w-10 h-10 rounded-full"
				class:bg-blue-600={step >= 3}
				class:bg-gray-300={step < 3}
				class:text-white={step >= 3}
			>
				3
			</div>
		</div>
	</div>

	<!-- Error Display -->
	{#if error}
		<div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded mb-4">
			{error}
		</div>
	{/if}

	<!-- Step 1: Upload & Map -->
	{#if step === 1}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 1: Upload CSV & Map Columns</h2>

			{#if !previewData}
				<div class="mb-4">
					<label class="block text-sm font-medium text-gray-700 mb-2">
						Select CSV File
					</label>
					<!-- svelte-ignore a11y_no_static_element_interactions -->
					<div
						class="relative border-2 border-dashed rounded-lg p-8 text-center cursor-pointer transition-colors {isDragging ? 'border-blue-500 bg-blue-50' : 'border-gray-300 bg-gray-50 hover:border-gray-400 hover:bg-gray-100'}"
						ondrop={handleDrop}
						ondragover={handleDragOver}
						ondragleave={handleDragLeave}
						onclick={() => fileInputRef?.click()}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') fileInputRef?.click(); }}
						role="button"
						tabindex="0"
					>
						<input
							bind:this={fileInputRef}
							type="file"
							accept=".csv"
							onchange={handleFileSelect}
							class="hidden"
						/>
						<svg class="mx-auto h-10 w-10 {isDragging ? 'text-blue-500' : 'text-gray-400'}" fill="none" stroke="currentColor" viewBox="0 0 24 24">
							<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
						</svg>
						{#if isDragging}
							<p class="mt-2 text-sm font-medium text-blue-600">Drop your CSV file here</p>
						{:else}
							<p class="mt-2 text-sm text-gray-600">
								<span class="font-medium text-blue-600 hover:text-blue-500">Click to choose a file</span>
								or drag and drop
							</p>
							<p class="mt-1 text-xs text-gray-500">CSV files up to 10 MB</p>
						{/if}
					</div>
				</div>
			{:else}
				<div class="mb-4">
					<p class="text-sm text-gray-600 mb-4">
						File: <span class="font-medium">{file?.name}</span> ({previewData.totalRows} rows)
					</p>

					<!-- Import Mode Selector -->
					<div class="bg-gray-50 rounded-lg p-4 mb-4">
						<div class="grid grid-cols-2 gap-4">
							<div>
								<label class="block text-sm font-medium text-gray-700 mb-1">Import Mode</label>
								<select
									bind:value={importMode}
									class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
								>
									<option value="create">Insert (create new records)</option>
									<option value="update">Update (modify existing records)</option>
									<option value="upsert">Upsert (create or update)</option>
									<option value="delete">Delete (remove records)</option>
								</select>
							</div>
							{#if importMode !== 'create'}
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1">
										Match Records By
										<span class="text-red-500">*</span>
									</label>
									<select
										bind:value={matchField}
										class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
									>
										<option value="">-- Select field --</option>
										{#each availableFields.filter(f => f.type === 'varchar' || f.type === 'email') as field}
											<option value={field.name}>{field.label || field.name}</option>
										{/each}
									</select>
									<p class="text-xs text-gray-500 mt-1">
										{#if importMode === 'update'}
											Records will be matched and updated by this field
										{:else if importMode === 'upsert'}
											Records will be matched by this field; new records created if no match
										{:else if importMode === 'delete'}
											Records matching this field value will be deleted
										{/if}
									</p>
								</div>
							{/if}
						</div>
					</div>

					<!-- Column Mapping Table (hidden for delete mode) -->
					{#if importMode === 'delete'}
						<div class="bg-gray-50 border border-gray-200 rounded-lg p-4">
							<p class="text-sm text-gray-600">
								In delete mode, only the <strong>Match Records By</strong> field above is used to identify records.
								No column mapping is needed.
							</p>
							{#if matchField && previewData?.headers}
								{@const matchHeader = previewData.headers.find((h: string) => h.toLowerCase() === matchField.toLowerCase())}
								{#if matchHeader}
									<p class="text-sm text-green-700 mt-2">
										CSV column "<strong>{matchHeader}</strong>" will be used to match records by <strong>{matchField === 'id' ? 'ID' : matchField}</strong>.
									</p>
								{:else}
									<p class="text-sm text-red-600 mt-2">
										No CSV column matches the selected field "<strong>{matchField}</strong>". Make sure your CSV has a column with that name.
									</p>
								{/if}
							{/if}
						</div>
					{:else}
					<div class="overflow-x-auto">
						<table class="min-w-full divide-y divide-gray-200">
							<thead class="bg-gray-50">
								<tr>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">CSV Header</th>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Sample Data</th>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Maps To</th>
									<th class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase">Options</th>
								</tr>
							</thead>
							<tbody class="bg-white divide-y divide-gray-200">
								{#each previewData.headers as header, idx}
									{@const mappedField = getFieldForMapping(columnMapping[header] || '')}
									{@const isLink = mappedField?.type === 'link'}
									<tr>
										<td class="px-4 py-3 text-sm font-medium text-gray-900">{header}</td>
										<td class="px-4 py-3 text-sm text-gray-600">
											{previewData.sampleRows[0]?.[header] || '-'}
										</td>
										<td class="px-4 py-3">
											<select
												bind:value={columnMapping[header]}
												onchange={() => updateMapping(header, columnMapping[header])}
												class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
											>
												<option value="">-- Skip this column --</option>
												{#each availableFields.filter(f => !(importMode === 'create' && f.name.toLowerCase() === 'id')) as field}
													<option value={field.name}>{field.label || field.name}</option>
												{/each}
											</select>
										</td>
										<td class="px-4 py-3">
											{#if isLink && mappedField}
												<div class="flex gap-2 items-start">
													<!-- Look up by field dropdown -->
													<div class="flex-1">
														<label class="block text-xs text-gray-500 mb-1">Look up by</label>
														<select
															value={lookupResolution[mappedField.name]?.matchField || ''}
															onchange={(e) => enableLookup(mappedField.name, e.currentTarget.value)}
															class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 text-sm"
														>
															<option value="">Use ID directly</option>
															{#if mappedField.relatedEntityFields && mappedField.relatedEntityFields.length > 0}
																{#each mappedField.relatedEntityFields as relField}
																	<option value={relField.name}>{relField.label || relField.name}</option>
																{/each}
															{:else}
																<option value="name">Name</option>
															{/if}
														</select>
													</div>
													<!-- If not found action dropdown -->
													{#if lookupResolution[mappedField.name]}
														<div class="flex-1">
															<label class="block text-xs text-gray-500 mb-1">If not found</label>
															<select
																value={lookupResolution[mappedField.name]?.createIfNotFound ? 'create' : 'fail'}
																onchange={(e) => setLookupNotFoundAction(mappedField.name, e.currentTarget.value as 'fail' | 'create')}
																class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 text-sm"
															>
																<option value="fail">Fail import</option>
																<option value="create">Create new {mappedField.relatedEntity || 'record'}</option>
															</select>
														</div>
													{/if}
												</div>
											{:else}
												<span class="text-gray-400 text-sm">—</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
					{/if}
				</div>

				<div class="flex justify-between mt-6">
					<button
						onclick={onCancel}
						class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
					>
						Cancel
					</button>
					<button
						onclick={analyzeData}
						disabled={loading || (importMode !== 'create' && !matchField)}
						class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
						title={importMode !== 'create' && !matchField ? 'Please select a field to match records by' : ''}
					>
						{loading ? 'Analyzing...' : 'Analyze Data'}
					</button>
				</div>
			{/if}
		</div>
	{/if}

	<!-- Step 2.5: Create Missing Lookups -->
	{#if step === 2.5 && missingLookups.length > 0}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Create New Records</h2>
			<p class="text-gray-600 mb-4">
				The following records will be created during import. Please fill in the required fields.
			</p>

			{#each missingLookups as lookup}
				<div class="border rounded-lg p-4 mb-4">
					<h3 class="font-medium text-gray-900 mb-2">
						New {lookup.relatedEntity}: <span class="text-blue-600">{lookup.matchValue}</span>
					</h3>
					<p class="text-sm text-gray-500 mb-3">
						Referenced by {lookup.rowIndices.length} row{lookup.rowIndices.length > 1 ? 's' : ''} in your CSV
					</p>

					{#if lookup.requiredFields && lookup.requiredFields.length > 0}
						<div class="grid grid-cols-2 gap-4">
							{#each lookup.requiredFields as reqField}
								<div>
									<label class="block text-sm font-medium text-gray-700 mb-1">
										{reqField.label || reqField.name} <span class="text-red-500">*</span>
									</label>
									<input
										type="text"
										value={newRecordData[lookup.fieldName]?.[lookup.matchValue]?.[reqField.name] || ''}
										oninput={(e) => updateNewRecordField(lookup.fieldName, lookup.matchValue, reqField.name, e.currentTarget.value)}
										class="block w-full border-gray-300 rounded-md shadow-sm focus:ring-blue-500 focus:border-blue-500 sm:text-sm"
										placeholder={`Enter ${reqField.label || reqField.name}`}
									/>
								</div>
							{/each}
						</div>
					{:else}
						<p class="text-sm text-green-600">No additional required fields needed.</p>
					{/if}
				</div>
			{/each}

			<div class="flex justify-between mt-6">
				<button
					onclick={goBack}
					class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
				>
					Back
				</button>
				<button
					onclick={proceedFromLookups}
					disabled={loading || !canProceedFromLookups()}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
					title={!canProceedFromLookups() ? 'Please fill in all required fields' : ''}
				>
					{loading ? 'Validating...' : 'Continue'}
				</button>
			</div>
		</div>
	{/if}

	<!-- Step 2: Validate -->
	{#if step === 2 && analyzeResult}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 2: Validation Results</h2>

			<!-- Summary -->
			<div class="grid grid-cols-3 gap-4 mb-6">
				<div class="bg-blue-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Total Rows</p>
					<p class="text-2xl font-bold text-blue-600">{analyzeResult.totalRows}</p>
				</div>
				<div class="bg-green-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Valid Rows</p>
					<p class="text-2xl font-bold text-green-600">{analyzeResult.validRows}</p>
				</div>
				<div class="bg-red-50 p-4 rounded-lg">
					<p class="text-sm text-gray-600">Invalid Rows</p>
					<p class="text-2xl font-bold text-red-600">{analyzeResult.invalidRows}</p>
				</div>
			</div>

			<!-- Missing Required Fields -->
			{#if analyzeResult.missingRequired.length > 0}
				<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4 mb-4">
					<h3 class="font-semibold text-yellow-800 mb-2">Missing Required Fields</h3>
					<ul class="list-disc list-inside text-yellow-700">
						{#each analyzeResult.missingRequired as field}
							<li>{field}</li>
						{/each}
					</ul>
				</div>
			{/if}

			<!-- Validation Issues -->
			{#if analyzeResult.issues.length > 0}
				<div class="mb-4">
					<h3 class="font-semibold mb-2">Validation Issues</h3>
					<div class="overflow-x-auto max-h-96 overflow-y-auto">
						<table class="min-w-full divide-y divide-gray-200">
							<thead class="bg-gray-50 sticky top-0">
								<tr>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Row</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Column</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Value</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Issue</th>
									<th class="px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase">Expected</th>
								</tr>
							</thead>
							<tbody class="bg-white divide-y divide-gray-200">
								{#each analyzeResult.issues as issue}
									<tr>
										<td class="px-4 py-2 text-sm">{issue.row}</td>
										<td class="px-4 py-2 text-sm font-medium">{issue.column}</td>
										<td class="px-4 py-2 text-sm text-gray-600">{issue.value || '(empty)'}</td>
										<td class="px-4 py-2 text-sm text-red-600">{issue.message}</td>
										<td class="px-4 py-2 text-sm text-gray-500">{issue.expected || '-'}</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>
			{/if}

			<!-- Actions -->
			<div class="flex justify-between mt-6">
				<button
					onclick={goBack}
					class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
				>
					Back
				</button>
				<div class="flex gap-2">
					<button
						onclick={skipDuplicateCheck}
						disabled={!analyzeResult.valid}
						class="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50 disabled:opacity-50"
						title={!analyzeResult.valid ? 'Fix validation issues first' : ''}
					>
						Skip Duplicate Check
					</button>
					<button
						onclick={checkDuplicates}
						disabled={loading || !analyzeResult.valid}
						class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50"
						title={!analyzeResult.valid ? 'Fix validation issues before continuing' : ''}
					>
						{loading ? 'Checking...' : 'Check for Duplicates'}
					</button>
				</div>
			</div>
		</div>
	{/if}

	<!-- All Clear Message -->
	{#if showAllClear}
		<div class="bg-green-50 border border-green-200 rounded-lg p-4 text-center">
			<svg class="w-8 h-8 text-green-500 mx-auto mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
			</svg>
			<p class="text-sm font-medium text-green-800">No duplicates detected</p>
			<p class="text-xs text-green-600 mt-1">All rows are unique. Proceeding to import...</p>
		</div>
	{/if}

	<!-- Duplicate Check Loading/Error States -->
	{#if checkingDuplicates}
		<div class="py-8 px-4">
			{#if dupCheckPhase === 'checking' && dupCheckTotal > 0}
				{@const pct = Math.round((dupCheckProcessed / dupCheckTotal) * 100)}
				<p class="text-sm font-medium text-gray-700 text-center mb-2">
					Checking row {dupCheckProcessed.toLocaleString()} of {dupCheckTotal.toLocaleString()}
				</p>
				<div class="w-full bg-gray-200 rounded-full h-2.5 mb-2">
					<div
						class="bg-blue-600 h-2.5 rounded-full transition-all duration-150"
						style="width: {pct}%"
					></div>
				</div>
				<div class="flex justify-between text-xs text-gray-500">
					<span>{pct}% complete</span>
					{#if dupCheckDuplicates > 0}
						<span>{dupCheckDuplicates} duplicate{dupCheckDuplicates !== 1 ? 's' : ''} found</span>
					{/if}
				</div>
			{:else if dupCheckPhase === 'preparing'}
				<div class="text-center">
					<div class="w-full bg-gray-200 rounded-full h-2.5 mb-2 overflow-hidden">
						<div class="bg-blue-500 h-2.5 rounded-full w-1/3 animate-pulse"></div>
					</div>
					<p class="text-sm text-gray-500">Preparing duplicate detection...</p>
				</div>
			{:else}
				<div class="text-center">
					<div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto"></div>
					<p class="text-sm text-gray-500 mt-3">Starting duplicate check...</p>
				</div>
			{/if}
		</div>
	{/if}

	{#if duplicateCheckError}
		<div class="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
			<p class="text-sm text-yellow-800 font-medium">Duplicate check failed</p>
			<p class="text-xs text-yellow-600 mt-1">{duplicateCheckError}</p>
			<div class="flex gap-2 mt-3">
				<button
					onclick={checkDuplicates}
					class="px-3 py-1.5 text-xs bg-yellow-100 text-yellow-800 rounded hover:bg-yellow-200"
				>Retry</button>
				<button
					onclick={skipDuplicateCheck}
					class="px-3 py-1.5 text-xs text-yellow-700 underline hover:text-yellow-900"
				>Skip and import anyway</button>
			</div>
		</div>
	{/if}

	<!-- Step 2.75: Duplicate Review -->
	{#if step === 2.75 && duplicateResult}
		<div class="space-y-6">
			<!-- Header with resolved count and bulk actions -->
			<div class="flex items-center justify-between">
				<div>
					<h3 class="text-lg font-semibold text-gray-900">Review Duplicates</h3>
					<p class="text-sm text-gray-500 mt-1">
						{getResolvedCount()} of {getTotalFlaggedCount()} rows resolved
						{#if duplicateResult.withinFileGroups?.length > 0}
							&middot; {duplicateResult.withinFileGroups.length} group{duplicateResult.withinFileGroups.length > 1 ? 's' : ''} of duplicate rows within file
						{/if}
					</p>
				</div>
				<div class="flex gap-2">
					<button
						onclick={() => bulkResolve('skip')}
						class="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
					>
						Skip All Remaining
					</button>
					<button
						onclick={() => bulkResolve('import')}
						class="px-3 py-1.5 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
					>
						Import All Remaining
					</button>
				</div>
			</div>

			<!-- Database matches - side-by-side -->
			{#each duplicateResult.databaseMatches || [] as match, idx}
				<div class="border rounded-lg overflow-hidden">
					<!-- Match header -->
					<div class="bg-gray-50 px-4 py-3 flex items-center justify-between border-b">
						<div class="flex items-center gap-3">
							<span class="text-sm font-medium text-gray-700">Row {match.importRowIndex + 1}</span>
							<span class="px-2 py-0.5 rounded text-xs font-medium {getConfidenceColor(match.confidenceTier)}">
								{Math.round(match.confidenceScore * 100)}% match
							</span>
							<span class="text-xs text-gray-400">via {match.ruleName}</span>
						</div>
						<div class="flex gap-1">
							<button
								class="px-2.5 py-1 text-xs rounded {resolutions.get(match.importRowIndex)?.action === 'skip' ? 'bg-gray-700 text-white' : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'}"
								onclick={() => setResolution(match.importRowIndex, 'skip', match.matchedRecordId)}
							>Skip</button>
							<button
								class="px-2.5 py-1 text-xs rounded {resolutions.get(match.importRowIndex)?.action === 'update' ? 'bg-blue-600 text-white' : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'}"
								onclick={() => setResolution(match.importRowIndex, 'update', match.matchedRecordId)}
							>Update Existing</button>
							<button
								class="px-2.5 py-1 text-xs rounded {resolutions.get(match.importRowIndex)?.action === 'import' ? 'bg-green-600 text-white' : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'}"
								onclick={() => setResolution(match.importRowIndex, 'import')}
							>Import Anyway</button>
							<button
								class="px-2.5 py-1 text-xs rounded bg-white border border-gray-300 text-gray-700 hover:bg-gray-50"
								onclick={() => {
									window.open(`/merge?entity=${entityName}&targetId=${match.matchedRecordId}`, '_blank');
									setResolution(match.importRowIndex, 'merge', match.matchedRecordId);
								}}
							>Merge &rarr;</button>
						</div>
					</div>

					<!-- Side-by-side comparison table -->
					<div class="grid grid-cols-2 divide-x">
						<!-- Import Row (Left) -->
						<div>
							<div class="bg-blue-50 px-4 py-2 text-xs font-medium text-blue-700 uppercase tracking-wider">Import Row</div>
							<div class="divide-y">
								{#each Object.entries(match.importRow) as [field, value]}
									<div class="px-4 py-2 flex justify-between text-sm {match.matchedFields?.includes(field) ? 'bg-yellow-50 border-l-3 border-l-yellow-400' : ''}">
										<span class="text-gray-500 font-medium">{field}</span>
										<span class="text-gray-900">{value ?? '(empty)'}</span>
									</div>
								{/each}
							</div>
						</div>

						<!-- Existing Record (Right) -->
						<div>
							<div class="bg-green-50 px-4 py-2 text-xs font-medium text-green-700 uppercase tracking-wider">Existing Record</div>
							<div class="divide-y">
								{#each Object.entries(match.matchedRecord) as [field, value]}
									<div class="px-4 py-2 flex justify-between text-sm {match.matchedFields?.includes(field) ? 'bg-yellow-50 border-l-3 border-l-yellow-400' : ''}">
										<span class="text-gray-500 font-medium">{field}</span>
										<span class="text-gray-900">{value ?? '(empty)'}</span>
									</div>
								{/each}
							</div>
						</div>
					</div>

					<!-- Other matches (expandable) -->
					{#if match.otherMatches && match.otherMatches.length > 0}
						<details class="border-t px-4 py-2">
							<summary class="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
								{match.otherMatches.length} other potential match{match.otherMatches.length > 1 ? 'es' : ''}
							</summary>
							<ul class="mt-2 space-y-1">
								{#each match.otherMatches as other}
									<li class="flex items-center justify-between text-sm">
										<span class="text-gray-700">{other.name} ({Math.round(other.score * 100)}%)</span>
										<button
											class="text-xs text-blue-600 hover:underline"
											onclick={() => setResolution(match.importRowIndex, resolutions.get(match.importRowIndex)?.action || 'skip', other.recordId)}
										>
											Select this match
										</button>
									</li>
								{/each}
							</ul>
						</details>
					{/if}
				</div>
			{/each}

			<!-- Within-file duplicate groups -->
			{#each duplicateResult.withinFileGroups || [] as group}
				{@const groupFields = [...new Set(group.rows.flatMap(r => Object.keys(r)))].sort()}
				{@const matchField = group.groupId.split(':')[0] || ''}
				{@const differingFields = new Set(groupFields.filter(f => {
					const vals = group.rows.map(r => r[f] ?? '');
					return !vals.every(v => v === vals[0]);
				}))}
				<div class="border rounded-lg overflow-hidden">
					<div class="bg-orange-50 px-4 py-3 border-b">
						<div class="flex items-center gap-2">
							<h4 class="text-sm font-medium text-orange-800">Duplicate Rows Within File</h4>
							<span class="px-2 py-0.5 rounded text-xs font-medium text-red-600 bg-red-50">100% match</span>
							<span class="text-xs text-gray-400">{group.rowIndices.length} rows</span>
							{#if differingFields.size > 0}
								<span class="text-xs text-orange-600">&middot; {differingFields.size} field{differingFields.size > 1 ? 's' : ''} differ</span>
							{/if}
						</div>
						<p class="text-xs text-orange-600 mt-1">These rows appear to be duplicates of each other. Select which one to keep, or import all:</p>
					</div>
					<div class="divide-y">
						<label class="flex items-start gap-3 px-4 py-3 cursor-pointer hover:bg-gray-50 {withinFileSelections.get(group.groupId) === -1 ? 'bg-green-50' : ''}">
							<input
								type="radio"
								name="group-{group.groupId}"
								checked={withinFileSelections.get(group.groupId) === -1}
								onchange={() => setWithinFileSelection(group.groupId, -1)}
								class="mt-1"
							/>
							<div class="flex-1 min-w-0">
								<span class="text-sm font-medium text-green-700">Keep All — Not Duplicates</span>
								<div class="text-xs text-gray-500 mt-0.5">Import all rows in this group</div>
							</div>
						</label>
						{#each group.rows as row, rowIdx}
							{@const isSelected = withinFileSelections.get(group.groupId) === group.rowIndices[rowIdx]}
							{@const nonEmptyEntries = Object.entries(row).filter(([_, v]) => v != null && v !== '')}
							<div class="border-b last:border-b-0 {isSelected ? 'bg-blue-50' : ''}">
								<label class="flex items-start gap-3 px-4 py-3 cursor-pointer hover:bg-gray-50">
									<input
										type="radio"
										name="group-{group.groupId}"
										checked={isSelected}
										onchange={() => setWithinFileSelection(group.groupId, group.rowIndices[rowIdx])}
										class="mt-1"
									/>
									<div class="flex-1 min-w-0">
										<span class="text-sm font-medium text-gray-900">Row {group.rowIndices[rowIdx] + 1}</span>
										<div class="text-xs text-gray-500 mt-0.5">
											{nonEmptyEntries.slice(0, 4).map(([k, v]) => `${k}: ${v}`).join(' | ')}
											{#if nonEmptyEntries.length > 4}
												<span class="text-gray-400"> +{nonEmptyEntries.length - 4} more</span>
											{/if}
										</div>
									</div>
								</label>
								<details class="px-4 pb-3 ml-10">
									<summary class="text-xs text-blue-600 cursor-pointer hover:text-blue-800 select-none">
										Show all {groupFields.length} fields
									</summary>
									<table class="mt-2 w-full text-xs">
										<tbody>
											{#each groupFields as field, fieldIdx}
												{@const value = row[field]}
												{@const isDiff = differingFields.has(field)}
												<tr class="{fieldIdx % 2 === 0 ? 'bg-gray-50' : ''} {isDiff ? 'border-l-2 border-l-amber-400' : ''}">
													<td class="py-1 pl-2 pr-4 font-medium whitespace-nowrap {isDiff ? 'text-amber-800' : 'text-gray-400'}" style="width: 40%">{field}</td>
													<td class="py-1 pr-2 {isDiff ? 'text-gray-900 font-medium' : value != null && value !== '' ? 'text-gray-600' : 'text-gray-300 italic'}">
														{#if value != null && value !== ''}
															{value}
														{:else}
															(empty)
														{/if}
													</td>
												</tr>
											{/each}
										</tbody>
									</table>
								</details>
							</div>
						{/each}
					</div>
				</div>
			{/each}

			<!-- Navigation -->
			<div class="flex justify-between items-center pt-4 border-t">
				<button
					onclick={() => { step = 2; duplicateResult = null; }}
					class="px-4 py-2 text-sm border border-gray-300 rounded-md hover:bg-gray-50"
				>
					Back
				</button>
				<button
					onclick={proceedToImport}
					disabled={!allResolved()}
					class="px-4 py-2 text-sm font-medium rounded-md {allResolved() ? 'bg-blue-600 text-white hover:bg-blue-700' : 'bg-gray-200 text-gray-500 cursor-not-allowed'}"
				>
					Proceed to Import ({getResolvedCount()} of {getTotalFlaggedCount()} resolved)
				</button>
			</div>
		</div>
	{/if}

	<!-- Step 3: Import Complete -->
	{#if step === 3 && importResult}
		<div class="bg-white shadow rounded-lg p-6">
			<h2 class="text-xl font-semibold mb-4">Step 3: Import Complete</h2>

			<div class="bg-green-50 border border-green-200 rounded-lg p-4 mb-4">
				<h3 class="font-semibold text-green-800 mb-3">Import Summary</h3>

				<!-- Action counts grid -->
				<div class="grid grid-cols-2 md:grid-cols-3 gap-3 mb-3">
					{#if importResult.created > 0}
						<div class="bg-white rounded-md p-3 border border-green-200">
							<div class="text-sm text-gray-600">Imported</div>
							<div class="text-2xl font-bold text-green-700">{importResult.created}</div>
						</div>
					{/if}
					{#if importResult.updated > 0}
						<div class="bg-white rounded-md p-3 border border-blue-200">
							<div class="text-sm text-gray-600">Updated</div>
							<div class="text-2xl font-bold text-blue-700">{importResult.updated}</div>
						</div>
					{/if}
					{#if importResult.skipped && importResult.skipped > 0}
						<div class="bg-white rounded-md p-3 border border-gray-200">
							<div class="text-sm text-gray-600">Skipped</div>
							<div class="text-2xl font-bold text-gray-700">{importResult.skipped}</div>
						</div>
					{/if}
					{#if importResult.merged && importResult.merged > 0}
						<div class="bg-white rounded-md p-3 border border-purple-200">
							<div class="text-sm text-gray-600">Sent to Merge</div>
							<div class="text-2xl font-bold text-purple-700">{importResult.merged}</div>
						</div>
					{/if}
					{#if importResult.deleted && importResult.deleted > 0}
						<div class="bg-white rounded-md p-3 border border-red-200">
							<div class="text-sm text-gray-600">Deleted</div>
							<div class="text-2xl font-bold text-red-700">{importResult.deleted}</div>
						</div>
					{/if}
					{#if importResult.failed > 0}
						<div class="bg-white rounded-md p-3 border border-red-300">
							<div class="text-sm text-gray-600">Failed</div>
							<div class="text-2xl font-bold text-red-700">{importResult.failed}</div>
						</div>
					{/if}
				</div>

				<p class="text-gray-600 text-sm mt-2">Total rows processed: {importResult.totalRows}</p>

				{#if importResult.importId}
					<p class="text-gray-500 text-xs mt-1 font-mono">Import ID: {importResult.importId}</p>
				{/if}

				{#if importResult.warnings && importResult.warnings.length > 0}
					<div class="mt-3 bg-amber-50 border border-amber-300 rounded-lg p-3">
						<div class="flex items-start gap-2">
							<svg class="w-5 h-5 text-amber-600 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
								<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
							</svg>
							<div>
								<p class="text-sm font-medium text-amber-800">Warnings</p>
								<ul class="mt-1 text-sm text-amber-700 list-disc list-inside">
									{#each importResult.warnings as warning}
										<li>{warning}</li>
									{/each}
								</ul>
							</div>
						</div>
					</div>
				{/if}

				<!-- Audit report download -->
				{#if importResult.auditReport && auditHeaders.length > 0}
					<div class="mt-4 border border-gray-200 rounded-lg p-4">
						<div class="flex items-center justify-between mb-3">
							<h4 class="text-sm font-medium text-gray-700">Audit Report</h4>
							<button
								onclick={() => downloadFilteredAuditReport()}
								class="inline-flex items-center px-3 py-2 border border-green-300 text-sm font-medium rounded-md text-green-700 bg-white hover:bg-green-50"
							>
								<svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
									<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
								</svg>
								Download
							</button>
						</div>

						<p class="text-xs text-gray-500 mb-2">Select which data fields to include (Row Number, Action, IDs, and Reason are always included):</p>

						<div class="flex items-center gap-3 mb-2">
							<button
								onclick={() => {
									const dataFields = auditHeaders.slice(5);
									selectedAuditFields = new Set(dataFields);
								}}
								class="text-xs text-blue-600 hover:text-blue-800"
							>Select All</button>
							<button
								onclick={() => { selectedAuditFields = new Set(); }}
								class="text-xs text-blue-600 hover:text-blue-800"
							>Deselect All</button>
						</div>

						<div class="flex flex-wrap gap-2">
							{#each auditHeaders.slice(5) as field}
								<label class="inline-flex items-center gap-1 px-2 py-1 rounded border text-xs cursor-pointer
									{selectedAuditFields.has(field) ? 'bg-blue-50 border-blue-300 text-blue-700' : 'bg-gray-50 border-gray-200 text-gray-500'}">
									<input
										type="checkbox"
										checked={selectedAuditFields.has(field)}
										onchange={() => {
											const next = new Set(selectedAuditFields);
											if (next.has(field)) next.delete(field);
											else next.add(field);
											selectedAuditFields = next;
										}}
										class="w-3 h-3"
									/>
									{field}
								</label>
							{/each}
						</div>
					</div>
				{/if}
			</div>

			{#if importResult.failed > 0 && importResult.errors}
				<div class="bg-red-50 border border-red-200 rounded-lg p-4 mb-4">
					<h3 class="font-semibold text-red-800 mb-2">Failed Records: {importResult.failed}</h3>
					<ul class="list-disc list-inside text-red-700 max-h-48 overflow-y-auto">
						{#each importResult.errors as err}
							<li>Row {err.index}: {err.error}</li>
						{/each}
					</ul>
				</div>
			{/if}

			<div class="flex justify-end mt-6">
				<button
					onclick={onComplete}
					class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700"
				>
					Done
				</button>
			</div>
		</div>
	{/if}

	<!-- Loading Overlay -->
	{#if loading}
		<div class="fixed inset-0 bg-black bg-opacity-25 flex items-center justify-center z-50">
			<div class="bg-white rounded-lg p-8 shadow-xl min-w-[350px] max-w-md">
				{#if loadingMessage}
					<div class="text-center">
						<p class="text-lg font-medium text-gray-800 mb-4">{loadingMessage}</p>
						<div class="w-full bg-gray-200 rounded-full h-2.5 mb-3 overflow-hidden">
							<div class="bg-blue-600 h-2.5 rounded-full animate-progress-bar"></div>
						</div>
						<p class="text-sm text-gray-500">This may take a moment for large files</p>
					</div>
				{:else}
					<div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
					<p class="mt-4 text-gray-600 text-center">Processing...</p>
				{/if}
			</div>
		</div>
	{/if}
</div>

<style>
	@keyframes progress-indeterminate {
		0% { width: 0%; margin-left: 0%; }
		50% { width: 60%; margin-left: 20%; }
		100% { width: 0%; margin-left: 100%; }
	}
	:global(.animate-progress-bar) {
		animation: progress-indeterminate 1.8s ease-in-out infinite;
	}
</style>
