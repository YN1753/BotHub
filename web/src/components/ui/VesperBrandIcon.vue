<script setup lang="ts">
import { useId } from 'vue'

const uid = useId()
</script>

<template>
  <svg viewBox="6 5 52 50" fill="none" aria-hidden="true">
    <defs>
      <clipPath :id="`vb-mochi-${uid}`">
        <path d="M 31.2 14.4 C 43.4 14.4, 50.4 24.5, 50.4 36.8 C 50.4 47.1, 42.8 52.4, 31.2 52.4 C 19.6 52.4, 12.0 47.1, 12.0 36.8 C 12.0 24.5, 19.0 14.4, 31.2 14.4 Z" />
      </clipPath>
      <clipPath :id="`vb-orbit-front-${uid}`">
        <rect x="0" y="35.6" width="64" height="24" />
      </clipPath>
      <radialGradient :id="`vb-body-${uid}`" cx="24.5" cy="21.5" r="33" fx="21.5" fy="18.5" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#FFFFFF" />
        <stop offset="54%" stop-color="#F9F0E6" />
        <stop offset="100%" stop-color="#DCC1A3" />
      </radialGradient>
      <radialGradient :id="`vb-sub-${uid}`" cx="39" cy="43" r="20" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#FFB36B" stop-opacity="0.52" />
        <stop offset="65%" stop-color="#FFB36B" stop-opacity="0.15" />
        <stop offset="100%" stop-color="#FFB36B" stop-opacity="0" />
      </radialGradient>
      <linearGradient :id="`vb-orbit-${uid}`" x1="31.2" y1="27.2" x2="31.2" y2="44.0" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#6A564E" />
        <stop offset="48%" stop-color="#DEA976" />
        <stop offset="100%" stop-color="#FFF2DC" />
      </linearGradient>
      <linearGradient :id="`vb-star-${uid}`" x1="44" y1="8.5" x2="56" y2="20.5" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stop-color="#FFFFFF" />
        <stop offset="52%" stop-color="#FFE5BF" />
        <stop offset="100%" stop-color="#F2B880" />
      </linearGradient>
    </defs>

    <path d="M 14.2 14.8 Q 14.2 17.2 16.6 17.2 Q 14.2 17.2 14.2 19.6 Q 14.2 17.2 11.8 17.2 Q 14.2 17.2 14.2 14.8 Z" fill="#F2B880" opacity="0.82" />

    <g transform="rotate(-22 31.2 35.6)">
      <ellipse cx="31.2" cy="35.6" rx="25.8" ry="8.2" :stroke="`url(#vb-orbit-${uid})`" stroke-width="2.4" />
    </g>

    <g :clip-path="`url(#vb-mochi-${uid})`">
      <path d="M 31.2 14.4 C 43.4 14.4, 50.4 24.5, 50.4 36.8 C 50.4 47.1, 42.8 52.4, 31.2 52.4 C 19.6 52.4, 12.0 47.1, 12.0 36.8 C 12.0 24.5, 19.0 14.4, 31.2 14.4 Z" :fill="`url(#vb-body-${uid})`" />
      <circle cx="39" cy="43" r="20" :fill="`url(#vb-sub-${uid})`" />
      <rect x="21.8" y="26.6" width="5.2" height="11.4" rx="2.6" fill="#101524" />
      <circle cx="23.7" cy="29.4" r="1.25" fill="#FFFFFF" opacity="0.95" />
      <rect x="35.4" y="26.6" width="5.2" height="11.4" rx="2.6" fill="#101524" />
      <circle cx="37.3" cy="29.4" r="1.25" fill="#FFFFFF" opacity="0.95" />
    </g>
    <path
      d="M 31.2 14.4 C 43.4 14.4, 50.4 24.5, 50.4 36.8 C 50.4 47.1, 42.8 52.4, 31.2 52.4 C 19.6 52.4, 12.0 47.1, 12.0 36.8 C 12.0 24.5, 19.0 14.4, 31.2 14.4 Z"
      stroke="rgba(100, 116, 139, 0.24)"
      stroke-width="0.9"
      fill="none"
    />

    <g transform="rotate(-22 31.2 35.6)" :clip-path="`url(#vb-orbit-front-${uid})`">
      <ellipse cx="31.2" cy="35.6" rx="25.8" ry="8.2" :stroke="`url(#vb-orbit-${uid})`" stroke-width="2.4" />
    </g>

    <path d="M 50.2 6.8 Q 50.2 14.5 57.9 14.5 Q 50.2 14.5 50.2 22.2 Q 50.2 14.5 42.5 14.5 Q 50.2 14.5 50.2 6.8 Z" :fill="`url(#vb-star-${uid})`" />
  </svg>
</template>
