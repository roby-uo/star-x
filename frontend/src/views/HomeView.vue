<template>
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else class="star-home">
    <div class="star-home__grid"></div>
    <div class="star-home__sunbeam"></div>

    <header class="star-header">
      <nav class="star-shell star-nav" aria-label="star-X 导航">
        <span class="star-nav__spacer" aria-hidden="true"></span>
        <div class="star-nav__actions">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="star-icon-button"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <button
            class="star-icon-button"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="star-console-button"
          >
            <span class="star-console-button__avatar">{{ userInitial }}</span>
            <span>{{ t('home.dashboard') }}</span>
            <Icon name="externalLink" size="sm" />
          </router-link>
          <router-link v-else to="/login" class="star-console-button">
            <span class="star-console-button__avatar">A</span>
            <span>{{ t('home.login') }}</span>
            <Icon name="externalLink" size="sm" />
          </router-link>
        </div>
      </nav>
    </header>

    <main class="star-shell star-main">
      <section class="star-hero" aria-labelledby="star-hero-title">
        <div class="star-hero__copy">
          <p class="star-hero__eyebrow">PRIVATE AI INFRASTRUCTURE</p>
          <h1 id="star-hero-title">{{ siteName }}</h1>
          <p class="star-hero__subtitle">{{ siteSubtitle }}</p>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="star-cta">
            {{ isAuthenticated ? t('home.goToDashboard') : '进入控制台' }}
            <Icon name="arrowRight" size="lg" :stroke-width="2" />
          </router-link>

          <div class="star-tags" aria-label="产品能力">
            <span><Icon name="swap" size="sm" />{{ t('home.tags.subscriptionToApi') }}</span>
            <span><Icon name="shield" size="sm" />{{ t('home.tags.stickySession') }}</span>
            <span><Icon name="chart" size="sm" />{{ t('home.tags.realtimeBilling') }}</span>
          </div>
        </div>

        <div class="star-hero__visual" aria-label="动态 AI API 网络示意图">
          <div class="star-orb" aria-hidden="true">
            <img class="star-orb__art" src="/brand/star-x-lightning-orb.png" alt="" />
            <div class="star-orb__pulse"></div>
            <div class="star-orb__scan"></div>
            <div class="star-orb__core"></div>
            <div class="star-orb__ring star-orb__ring--one"></div>
            <div class="star-orb__ring star-orb__ring--two"></div>
            <svg class="star-orb__lightning" viewBox="0 0 460 460" fill="none">
              <path class="bolt bolt--one" d="M38 239 108 206l-29 47 83-42-36 70 105-72-45 88 113-72-58 108 124-55" />
              <path class="bolt bolt--two" d="m99 95 51 70-16-44 88 74-27-55 94 69-33-18 88 91" />
              <path class="bolt bolt--three" d="m109 371 67-70-25 49 101-68-43 67 101-66-29 50 78-33" />
              <path class="bolt bolt--four" d="m232 35-18 92 35-35-7 112 34-64 8 91" />
            </svg>
          </div>

          <div class="star-terminal">
            <div class="star-terminal__header">
              <span class="terminal-dot terminal-dot--red"></span>
              <span class="terminal-dot terminal-dot--yellow"></span>
              <span class="terminal-dot terminal-dot--green"></span>
              <span class="star-terminal__title">terminal</span>
            </div>
            <div class="star-terminal__body">
              <p><b>$</b> <em>curl</em> -X POST <i>/v1/messages</i></p>
              <p class="terminal-muted"># Routing to upstream...</p>
              <p><strong>200 OK</strong> <span>{ "content": "Hello!" }</span></p>
              <p><b>$</b> <span class="terminal-cursor"></span></p>
            </div>
          </div>
        </div>
      </section>

      <section class="star-feature-grid" aria-label="star-X 核心能力">
        <article class="star-feature-card">
          <div class="star-feature-card__icon star-feature-card__icon--blue"><Icon name="server" size="lg" /></div>
          <h2>{{ t('home.features.unifiedGateway') }}</h2>
          <p>{{ t('home.features.unifiedGatewayDesc') }}</p>
        </article>
        <article class="star-feature-card">
          <div class="star-feature-card__icon star-feature-card__icon--violet">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M16 20v-1.1c0-1.7-1.8-3.1-4-3.1s-4 1.4-4 3.1V20"/><circle cx="12" cy="8" r="3"/><path d="M19 20v-1c0-1.2-.8-2.3-2-2.8M17 5.3a3 3 0 0 1 0 5.4M5 20v-1c0-1.2.8-2.3 2-2.8M7 5.3a3 3 0 0 0 0 5.4"/></svg>
          </div>
          <h2>{{ t('home.features.multiAccount') }}</h2>
          <p>{{ t('home.features.multiAccountDesc') }}</p>
        </article>
        <article class="star-feature-card">
          <div class="star-feature-card__icon star-feature-card__icon--purple"><Icon name="creditCard" size="lg" /></div>
          <h2>{{ t('home.features.balanceQuota') }}</h2>
          <p>{{ t('home.features.balanceQuotaDesc') }}</p>
        </article>
      </section>

      <section class="star-providers" aria-labelledby="star-providers-title">
        <h2 id="star-providers-title">{{ t('home.providers.title') }}</h2>
        <p>{{ t('home.providers.description') }}</p>
        <div class="star-provider-list">
          <div class="star-provider"><b class="provider-mark provider-mark--orange">C</b><span>{{ t('home.providers.claude') }}</span><small>{{ t('home.providers.supported') }}</small></div>
          <div class="star-provider"><b class="provider-mark provider-mark--green">G</b><span>GPT</span><small>{{ t('home.providers.supported') }}</small></div>
          <div class="star-provider"><b class="provider-mark provider-mark--blue">G</b><span>{{ t('home.providers.gemini') }}</span><small>{{ t('home.providers.supported') }}</small></div>
          <div class="star-provider"><b class="provider-mark provider-mark--pink">A</b><span>{{ t('home.providers.antigravity') }}</span><small>{{ t('home.providers.supported') }}</small></div>
          <div class="star-provider star-provider--muted"><b class="provider-mark provider-mark--gray">+</b><span>{{ t('home.providers.more') }}</span><small>{{ t('home.providers.soon') }}</small></div>
        </div>
      </section>
    </main>

    <footer class="star-footer">
      <p>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'star-X')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '你的私人 AI API 网关')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => authStore.user?.email?.charAt(0).toUpperCase() || 'A')
const currentYear = computed(() => new Date().getFullYear())

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
})
</script>

<style scoped>
.star-home {
  --ink: #10152a;
  --muted: #4d5874;
  --line: rgba(114, 139, 196, 0.16);
  position: relative;
  min-height: 100vh;
  overflow: hidden;
  color: var(--ink);
  background: radial-gradient(circle at 72% 18%, rgba(126, 164, 255, 0.3), transparent 30%), linear-gradient(125deg, #fff 2%, #f7f8ff 42%, #edf1ff 100%);
  isolation: isolate;
}
.star-home__grid { position: absolute; inset: 0; z-index: -2; opacity: .62; background-image: linear-gradient(var(--line) 1px, transparent 1px), linear-gradient(90deg, var(--line) 1px, transparent 1px); background-size: 64px 64px; mask-image: linear-gradient(to bottom, #000 0%, #000 70%, transparent 100%); }
.star-home__sunbeam { position: absolute; z-index: -1; top: -18rem; right: 5%; width: 68rem; height: 56rem; border-radius: 50%; background: conic-gradient(from 210deg, transparent, rgba(255,255,255,.88), transparent 32%); filter: blur(22px); opacity: .7; transform: rotate(-12deg); }
.star-shell { width: min(1400px, calc(100% - 80px)); margin: 0 auto; }
.star-header { padding: 26px 0 0; }
.star-nav { display: flex; align-items: center; justify-content: space-between; min-height: 44px; }
.star-nav__spacer { width: 180px; }
.star-nav__actions { display: flex; align-items: center; gap: 9px; }
.star-icon-button { display: grid; width: 36px; height: 36px; place-items: center; border-radius: 50%; color: #526078; transition: .2s ease; }
.star-icon-button:hover { color: #111a32; background: rgba(255,255,255,.72); box-shadow: 0 6px 20px rgba(54,76,124,.11); }
.star-console-button { display: inline-flex; align-items: center; gap: 8px; padding: 5px 12px 5px 5px; border-radius: 999px; background: #11182b; color: white; font-size: 14px; font-weight: 650; box-shadow: 0 8px 22px rgba(19,28,52,.16); transition: transform .2s ease, box-shadow .2s ease; }
.star-console-button:hover { transform: translateY(-2px); box-shadow: 0 12px 26px rgba(19,28,52,.25); }
.star-console-button__avatar { display: grid; width: 26px; height: 26px; place-items: center; border-radius: 50%; background: linear-gradient(135deg,#4076ff,#2457e7); font-size: 12px; }
.star-main { padding: 44px 0 44px; }
.star-hero { display: grid; grid-template-columns: .82fr 1.18fr; align-items: center; min-height: 420px; gap: 24px; }
.star-hero__copy { position: relative; z-index: 2; padding-left: 8.2%; }
.star-hero__eyebrow { margin: 0 0 11px; color: #3776f1; font-size: 11px; font-weight: 750; letter-spacing: .16em; }
.star-hero h1 { margin: 0; color: #080d1d; font-size: clamp(72px, 8.2vw, 136px); line-height: .85; letter-spacing: -.085em; font-weight: 800; }
.star-hero__subtitle { margin: 25px 0 29px; color: #151c30; font-size: clamp(20px, 1.75vw, 31px); letter-spacing: .03em; }
.star-cta { display: inline-flex; align-items: center; gap: 15px; padding: 14px 27px; border-radius: 17px; color: white; font-size: 18px; font-weight: 650; background: linear-gradient(135deg,#4383ff,#2356e6); box-shadow: 0 13px 20px rgba(42, 102, 239, .27), inset 0 1px 1px rgba(255,255,255,.35); transition: transform .2s ease, box-shadow .2s ease; }
.star-cta:hover { transform: translateY(-3px); box-shadow: 0 18px 28px rgba(42,102,239,.35), inset 0 1px 1px rgba(255,255,255,.35); }
.star-tags { display: flex; flex-wrap: wrap; gap: 25px; margin-top: 48px; color: #343e55; font-size: 14px; font-weight: 600; }
.star-tags span { display: inline-flex; align-items: center; gap: 8px; }
.star-tags :deep(svg) { color: #4281f4; }
.star-hero__visual { position: relative; min-height: 420px; }
.star-orb { position: absolute; z-index: 1; top: -72px; right: 1%; width: min(560px, 46vw); aspect-ratio: 1; filter: drop-shadow(0 0 23px rgba(98,145,255,.42)); animation: orb-float 8s ease-in-out infinite; }
.star-orb__art { position: absolute; z-index: 1; inset: 0; width: 100%; height: 100%; object-fit: contain; animation: orb-electricity 4.8s ease-in-out infinite; }
.star-orb__pulse { position: absolute; z-index: 2; inset: 9%; border: 1px solid rgba(202,229,255,.55); border-radius: 50%; box-shadow: 0 0 18px rgba(133,190,255,.46), inset 0 0 23px rgba(80,142,255,.16); opacity: .16; pointer-events: none; animation: orb-pulse 3.8s ease-in-out infinite; }
.star-orb__scan { position: absolute; z-index: 2; inset: 11%; border-radius: 50%; background: linear-gradient(114deg, transparent 38%, rgba(222,241,255,.68) 48%, rgba(99,168,255,.22) 52%, transparent 62%); filter: blur(9px); opacity: .22; pointer-events: none; animation: orb-scan 5.4s linear infinite; }
.star-orb::before, .star-orb::after, .star-orb__core, .star-orb__ring { display: none; }
.star-orb__lightning { display: none; }
.bolt { stroke: #f8fbff; stroke-width: 2.2; stroke-linecap: round; stroke-linejoin: round; stroke-dasharray: 760; opacity: .8; animation: lightning 5.4s ease-in-out infinite; }
.bolt--two { animation-delay: 1.2s; stroke-width: 1.6; }.bolt--three { animation-delay: 2.6s; stroke-width: 1.4; }.bolt--four { animation-delay: 3.8s; stroke-width: 1.8; }
.star-terminal { position: absolute; z-index: 3; top: 100px; left: 7%; width: min(430px, 38vw); overflow: hidden; border: 1px solid rgba(255,255,255,.15); border-radius: 18px; background: linear-gradient(145deg,rgba(22,33,54,.96),rgba(7,16,30,.95)); box-shadow: 0 24px 45px rgba(31,57,110,.3), 0 0 0 1px rgba(10,26,54,.5), inset 0 1px 0 rgba(255,255,255,.12); transform: rotate(-.3deg); animation: terminal-float 7s ease-in-out infinite; }
.star-terminal__header { display: flex; align-items: center; gap: 8px; padding: 14px 17px; background: rgba(33,47,73,.7); border-bottom: 1px solid rgba(255,255,255,.05); }
.terminal-dot { width: 12px; height: 12px; border-radius: 50%; }.terminal-dot--red{background:#ff5c55}.terminal-dot--yellow{background:#f9bd4e}.terminal-dot--green{background:#2dd46f}.star-terminal__title { flex: 1; margin-right: 46px; color: #8a94a8; font-family: ui-monospace, monospace; font-size: 12px; text-align: center; }
.star-terminal__body { padding: 22px 27px 20px; color: #d5e1ff; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: clamp(12px, 1.05vw, 15px); line-height: 1.95; }.star-terminal__body p{margin:0}.star-terminal__body b{color:#16c784}.star-terminal__body em{color:#1acac8;font-style:normal}.star-terminal__body i{color:#53b9f5;font-style:normal}.star-terminal__body strong{display:inline-block;padding:0 7px;border-radius:3px;background:rgba(39,196,113,.23);color:#3bea92}.star-terminal__body span{color:#f2c56c}.terminal-muted{color:#8997b0;font-style:italic}.terminal-cursor{display:inline-block;width:8px;height:16px;margin-left:4px;vertical-align:-2px;background:#1ed18a;animation: cursor-blink 1s step-end infinite;}
.star-feature-grid { display: grid; grid-template-columns: repeat(3,1fr); gap: 24px; margin-top: 5px; }
.star-feature-card { min-height: 185px; padding: 25px 28px; border: 1px solid rgba(151,171,219,.22); border-radius: 18px; background: rgba(255,255,255,.64); box-shadow: 0 14px 34px rgba(65,91,156,.07), inset 0 1px 0 rgba(255,255,255,.9); backdrop-filter: blur(10px); transition: transform .25s ease, box-shadow .25s ease; }.star-feature-card:hover{transform:translateY(-6px);box-shadow:0 23px 40px rgba(65,91,156,.16)}.star-feature-card h2{margin:13px 0 11px;font-size:22px;font-weight:750}.star-feature-card p{max-width:34rem;margin:0;color:var(--muted);font-size:14px;line-height:1.7}.star-feature-card__icon{display:grid;width:50px;height:50px;place-items:center;border-radius:14px;color:white;box-shadow:0 9px 16px rgba(41,83,190,.24)}.star-feature-card__icon svg{width:26px;height:26px}.star-feature-card__icon--blue{background:linear-gradient(135deg,#20a1f4,#326cf1)}.star-feature-card__icon--violet{background:linear-gradient(135deg,#6686ff,#4e62ec)}.star-feature-card__icon--purple{background:linear-gradient(135deg,#b855f6,#7129e7)}
.star-providers { padding: 37px 0 26px; text-align: center; }.star-providers h2{margin:0;color:#080d1d;font-size:30px;font-weight:780}.star-providers>p{margin:7px 0 20px;color:#5c6680;font-size:14px}.star-provider-list{display:flex;flex-wrap:wrap;justify-content:center;gap:12px}.star-provider{display:flex;align-items:center;gap:9px;padding:10px 14px;border:1px solid rgba(151,171,219,.19);border-radius:11px;background:rgba(255,255,255,.62);box-shadow:0 7px 18px rgba(65,91,156,.06);font-size:14px;color:#27314a}.star-provider small{padding:2px 5px;border-radius:4px;background:#eef4ff;color:#3776e7;font-size:10px}.provider-mark{display:grid;width:27px;height:27px;place-items:center;border-radius:7px;color:white}.provider-mark--orange{background:#fa8b22}.provider-mark--green{background:#14a963}.provider-mark--blue{background:#2d87ea}.provider-mark--pink{background:#ea2461}.provider-mark--gray{background:#7b8497}.star-provider--muted{opacity:.67}.star-provider--muted small{background:#f0f1f5;color:#70798d}
.star-footer { padding: 20px; color:#67718a; font-size:12px; text-align:center; }
:global(.dark) .star-home { --ink:#edf3ff; --muted:#aebbd2; --line:rgba(133,158,210,.09); background:radial-gradient(circle at 70% 16%,rgba(67,105,209,.4),transparent 31%),linear-gradient(130deg,#080e1d,#10182d 48%,#182447); }:global(.dark) .star-hero h1,:global(.dark) .star-hero__subtitle,:global(.dark) .star-feature-card h2,:global(.dark) .star-providers h2{color:#f5f8ff}:global(.dark) .star-feature-card,:global(.dark) .star-provider{background:rgba(18,29,55,.69);border-color:rgba(153,178,237,.18)}:global(.dark) .star-tags,:global(.dark) .star-providers>p{color:#bfc9dc}
@keyframes orb-float{50%{translate:0 -12px;scale:1.015}}@keyframes orb-electricity{0%,100%{filter:brightness(1) saturate(1)}22%{filter:brightness(1.12) saturate(1.13)}25%{filter:brightness(.98) saturate(.96)}58%{filter:brightness(1.08) saturate(1.08)}}@keyframes orb-pulse{0%,100%{transform:scale(.96);opacity:.1}45%{transform:scale(1.035);opacity:.33}64%{transform:scale(1);opacity:.17}}@keyframes orb-scan{0%{transform:translate(-36%,-22%) rotate(14deg);opacity:0}12%{opacity:.24}48%{opacity:.28}76%{opacity:.08}100%{transform:translate(36%,22%) rotate(14deg);opacity:0}}@keyframes orbit-spin{to{transform:rotate(360deg)}}@keyframes core-pulse{50%{transform:scale(1.15);opacity:.72}}@keyframes ring-one{50%{transform:rotate(-14deg) scale(1.04)}}@keyframes ring-two{50%{transform:rotate(26deg) scale(.95)}}@keyframes lightning{0%,100%{stroke-dashoffset:0;opacity:.62}18%,28%{stroke-dashoffset:0;opacity:1}32%{stroke-dashoffset:80;opacity:.72}45%{stroke-dashoffset:0;opacity:.58}}@keyframes terminal-float{50%{transform:translateY(-8px) rotate(.3deg)}}@keyframes cursor-blink{50%{opacity:0}}
@media (prefers-reduced-motion: reduce){.star-orb,.star-orb__art,.star-orb__pulse,.star-orb__scan,.star-terminal,.terminal-cursor{animation:none}}
@media (max-width: 960px){.star-shell{width:min(100% - 40px,720px)}.star-nav__spacer{display:none}.star-nav{justify-content:flex-end}.star-hero{grid-template-columns:1fr;min-height:0;padding-top:35px}.star-hero__copy{padding-left:0;text-align:center}.star-tags{justify-content:center}.star-hero__visual{min-height:590px;margin-top:4px}.star-orb{top:-12px;right:50%;width:min(370px,76vw);transform:translateX(50%)}.star-terminal{top:333px;left:50%;width:min(430px,calc(100vw - 48px));transform:translateX(-50%);animation:none}.star-feature-grid{grid-template-columns:1fr}.star-feature-card{min-height:0}.star-main{padding-top:16px}}@media (max-width:560px){.star-header{padding-top:12px}.star-shell{width:calc(100% - 28px)}.star-nav__actions{gap:2px}.star-console-button{padding-right:9px}.star-console-button span:not(.star-console-button__avatar){display:none}.star-hero h1{font-size:70px}.star-hero__subtitle{font-size:20px}.star-tags{gap:14px;font-size:12px}.star-hero__visual{min-height:510px;margin-top:0}.star-orb{top:-4px;width:min(295px,76vw)}.star-terminal{top:258px;width:calc(100vw - 40px);border-radius:13px}.star-terminal__body{padding:15px 18px;font-size:11px}.star-terminal__header{padding:10px 12px}.terminal-dot{width:9px;height:9px}.star-provider-list{gap:8px}.star-provider{padding:8px 10px}.star-provider small{display:none}.star-feature-grid{gap:14px}.star-feature-card{padding:20px}.star-providers h2{font-size:26px}}
</style>
