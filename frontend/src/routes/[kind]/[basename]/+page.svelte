<script lang="ts">
  import { onMount } from 'svelte'

  import Header from '$lib/components/Header.svelte'
  import Footer from '$lib/components/Footer.svelte'
  import Popup from '$lib/components/Popup.svelte'
  import Loading from '$lib/components/Loading.svelte'
  import ErrorNotification from '$lib/components/ErrorNotification.svelte'

  import StateButton from '$lib/components/StateButton.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import ShowPrefixCheck from '$lib/components/ShowPrefixCheck.svelte'
  import WithDefaultCheck from '$lib/components/WithDefaultCheck.svelte'
  import CrossBrowser from '$lib/components/crossBrowser.svelte'
  import Pagination from './Pagination.svelte'

  import type { PathDef } from '$lib/structure'
  import { markFilter, markRender, toLower, kindView } from '$lib/components/functions'
	import { defaultStore, paginated, prefixStore, searchStore, stateStore, total, yangPaths } from './store'
	import type { FetchResponseMessage } from '$lib/workers/structure';

  // DEFAULTS
  let popupDetail = {}
  let paths: PathDef[] = []
  let workerComplete = false
  let workerStatus = {status: 404, error: {message: "Unknown Error"}}

  // BASENAME WORKER
  let basenameWorker: Worker | undefined = undefined
  async function loadWorker(kind: string, basename: string) {
    const BasenameWorker = await import('$lib/workers/fetch.worker?worker')
    basenameWorker = new BasenameWorker.default()
    basenameWorker.postMessage({kind, basename})
    basenameWorker.onmessage = onWorkerMessage
  }
  function onWorkerMessage(event: MessageEvent<FetchResponseMessage>) {
    const response = event.data
    workerStatus.error.message = response.message
    if(event.data.success) {
      paths = response.paths
      workerStatus.status = 200
    }
    workerComplete = true
  }
  
  // ON PAGELOAD
  export let data
  let {kind, basename, urlPath} = data
  onMount(() => loadWorker(kind, basename))

  // OTHER BINDING VARIABLES
  let searchInput = urlPath
  let stateInput = ""
  let showPathPrefix = false
  let pathWithDefault = false

  $: searchStore.set(toLower(searchInput))
  $: stateStore.set(stateInput)
  $: prefixStore.set(showPathPrefix)
  $: defaultStore.set(pathWithDefault)
  $: yangPaths.set(paths)
</script>

<svelte:head>
	<title>Yang Path Browser {basename} ({kindView(kind)})</title>
</svelte:head>

{#if !workerComplete}
  <Loading/>
{:else}
  {#if workerStatus.status === 200}
    <Header {kind} {basename} />
    <div class="min-w-[280px] overflow-x-auto font-nunito dark:bg-gray-800 pt-[75px] lg:pt-[85px]">
      <div class="px-6 pt-6 container mx-auto">
        <div class="flex items-center justify-between">
          <p class="text-gray-800 dark:text-gray-300">Path Browser</p>
          <CrossBrowser {kind} {basename} isTree={false} />
        </div>
        <SearchInput bind:searchInput />
        <div class="overflow-x-auto scroll-light dark:scroll-dark">
          <div class="py-2 space-x-2 flex items-center">
            <StateButton bind:stateInput />
            <ShowPrefixCheck bind:showPathPrefix />
            <WithDefaultCheck bind:pathWithDefault />
          </div>
        </div>
        <Pagination />
        <div class="overflow-x-auto rounded-t-lg max-w-full mt-2">
          <table class="text-left w-full">
            <colgroup>
              <col span="1" class="w-[2%]">
              <col span="1" class="w-[80%]">
              <col span="1" class="w-[18%]">
            </colgroup>
            <thead class="text-sm text-gray-800 dark:text-gray-300 bg-gray-300 dark:bg-gray-700">
              <tr>
                <th scope="col" class="px-3 py-2"></th>
                <th scope="col" class="px-3 py-2">Path</th>
                <th scope="col" class="px-3 py-2">Type</th>
              </tr>
            </thead>
            <tbody>
              {#if $total > 0}
                {#each $paginated as item}
                  {@const path = markFilter((showPathPrefix ? item["path-with-prefix"] : item.path), $searchStore)}
                  {@const type = markFilter(item.type, $searchStore)}
                  <tr class="bg-white dark:bg-gray-800 border-b dark:border-gray-700 text-gray-700 dark:text-gray-300 hover:cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-600" on:click={() => popupDetail = item}>
                    <td class="px-3 py-1.5 font-fira text-[13px] tracking-tight">{item["is-state"]}</td>
                    <td class="px-3 py-1.5 font-fira text-[13px] tracking-tight group"><div use:markRender={path}></div></td>
                    <td class="px-3 py-1.5 font-fira text-[13px] tracking-tight"><div use:markRender={type}></td>
                  </tr>
                {/each}
              {:else}
                <tr class="bg-white border-b dark:bg-gray-800 dark:border-gray-700">
                  <td colspan="3" class="px-3 py-1.5 font-fira text-[13px] text-red-600 text-center">No results found</td>
                </tr>
              {/if}
            </tbody>
          </table>
        </div>
        <Pagination />
        <Popup {kind} {basename} {popupDetail} />
      </div>
      <Footer home={false}/>
    </div>
  {:else}
    <ErrorNotification pageError={workerStatus} />
  {/if}
{/if}