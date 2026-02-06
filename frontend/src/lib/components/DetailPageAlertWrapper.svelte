<script lang="ts">
    import { onMount } from 'svelte';
    import { getPendingAlert, resolveAlert, type PendingAlert, type AlertResolution } from '$lib/api/dedup';
    import DuplicateAlertBanner from './DuplicateAlertBanner.svelte';
    import DuplicateWarningModal from './DuplicateWarningModal.svelte';
    import { toast } from '$lib/stores/toast.svelte';

    interface Props {
        entityType: string;
        recordId: string;
    }

    let { entityType, recordId }: Props = $props();

    let pendingAlert = $state<PendingAlert | null>(null);
    let loading = $state(true);
    let showModal = $state(false);

    async function loadAlert() {
        loading = true;
        try {
            pendingAlert = await getPendingAlert(entityType, recordId);
        } catch (error: any) {
            console.error('Failed to load pending alert:', error);
            // Don't show error toast - silent failure is fine here
        } finally {
            loading = false;
        }
    }

    async function handleDismiss() {
        if (!pendingAlert) return;

        try {
            await resolveAlert(entityType, recordId, 'dismissed');
            pendingAlert = null;
            showModal = false;
            toast.success('Alert dismissed');
        } catch (error: any) {
            console.error('Failed to dismiss alert:', error);
            toast.error('Failed to dismiss alert');
        }
    }

    function handleViewMatches() {
        showModal = true;
    }

    function handleCloseModal() {
        showModal = false;
    }

    function handleMergeComplete() {
        // After merge, the alert is resolved and we may have navigated to another record
        pendingAlert = null;
        showModal = false;
    }

    onMount(() => {
        loadAlert();
    });

    // Reload alert when recordId changes (navigating between records)
    $effect(() => {
        if (recordId) {
            loadAlert();
        }
    });
</script>

<!-- Only render if we have an alert (avoid flicker during loading) -->
{#if !loading && pendingAlert}
    <DuplicateAlertBanner
        alert={pendingAlert}
        onViewMatches={handleViewMatches}
        onDismiss={handleDismiss}
    />

    {#if showModal}
        <DuplicateWarningModal
            alert={pendingAlert}
            {entityType}
            currentRecordId={recordId}
            isBlockMode={pendingAlert.isBlockMode}
            onClose={handleCloseModal}
            onDismiss={handleDismiss}
            onMergeComplete={handleMergeComplete}
        />
    {/if}
{/if}
