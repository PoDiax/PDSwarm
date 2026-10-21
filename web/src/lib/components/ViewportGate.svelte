<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    minHeight?: string;
    rootMargin?: string;
    keepMounted?: boolean;
    children?: Snippet;
  }

  let {
    minHeight = "76px",
    rootMargin = "350px 0px",
    keepMounted = false,
    children,
  }: Props = $props();

  let container = $state<HTMLDivElement | null>(null);
  let isIntersecting = $state(false);
  let hasIntersected = $state(false);

  let shouldRender = $derived(
    keepMounted ? (hasIntersected || isIntersecting) : isIntersecting
  );

  $effect(() => {
    if (!container) return;
    if (typeof IntersectionObserver === "undefined") {
      isIntersecting = true;
      hasIntersected = true;
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (entry) {
          isIntersecting = entry.isIntersecting;
          if (entry.isIntersecting) {
            hasIntersected = true;
          }
        }
      },
      { rootMargin }
    );

    observer.observe(container);
    return () => observer.disconnect();
  });
</script>

<div
  bind:this={container}
  style:min-height={shouldRender ? undefined : minHeight}
  class="transition-opacity duration-150"
>
  {#if shouldRender}
    {@render children?.()}
  {:else}
    <!-- Lightweight skeleton placeholder while offscreen -->
    <div
      class="w-full rounded-2xl bg-[#12131c]/60 border border-white/[0.04]"
      style:height={minHeight}
    ></div>
  {/if}
</div>
