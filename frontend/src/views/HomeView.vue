<template>
  <div v-if="homeContent" class="min-h-screen">
    <iframe v-if="isHomeContentUrl" :src="homeContent.trim()" class="h-screen w-full border-0" allowfullscreen />
    <div v-else v-html="homeContent"></div>
  </div>

  <div v-else class="starx-home" :class="{ 'is-dark': isDark }">
    <header class="site-header">
      <nav class="nav-shell" aria-label="主导航">
        <router-link to="/home" class="brand" aria-label="star-X 首页">
          <img src="/starx-logo-transparent.png" alt="star-X" class="brand-logo" />
        </router-link>
        <div class="nav-links">
          <a class="active" href="#top">首页</a>
          <router-link :to="dashboardPath">控制台</router-link>
          <a href="#models">模型广场</a>
          <a href="#metrics">排行榜</a>
          <a :href="docUrl || '#quick-start'" :target="docUrl ? '_blank' : undefined" rel="noopener noreferrer">文档</a>
        </div>
        <div class="nav-actions">
          <LocaleSwitcher class="locale-switcher" />
          <button class="icon-button" type="button" :aria-label="isDark ? '切换浅色模式' : '切换深色模式'" @click="toggleTheme">
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="login-button">
            {{ isAuthenticated ? '进入控制台' : '登录' }}
          </router-link>
        </div>
      </nav>
    </header>

    <main id="top">
      <section class="hero section-shell">
        <div class="hero-copy">
          <div class="eyebrow"><span></span> 未来 AI 基础设施</div>
          <div class="hero-wordmark" role="img" aria-label="star-X"><span>star-</span><b>X</b></div>
          <h2>你的私人 <strong>AI API</strong> 网关</h2>
          <p>通过统一、标准的接口协议接入海量模型。<br />承载 AI 应用，高效管理数字资产，连接未来。</p>
          <div class="hero-actions">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="primary-button">{{ isAuthenticated ? '进入控制台' : '进入控制台' }} <b>→</b></router-link>
          </div>
          <div class="trust-list" aria-label="产品能力">
            <span>◌ 订阅 API</span><span>♧ 会话保持</span><span>♧ 按量计费</span>
          </div>
        </div>

        <div class="api-panel" aria-label="API 请求示例">
          <div class="api-tabs">
            <button v-for="tab in apiTabs" :key="tab" type="button" :class="{ active: activeApiTab === tab }" @click="activeApiTab = tab">{{ tab }}</button>
            <span class="api-status"><i></i> 200 OK</span>
          </div>
          <div class="endpoint"><span>POST</span> /v1/messages</div>
          <div class="code-block">
            <p class="label">REQUEST</p>
            <pre><code><b>curl</b> -X <strong>POST</strong> <i>"/v1/messages"</i> \
  -H <i>"x-api-key: sk-********"</i> \
  -H <i>"anthropic-version: 2023-06-01"</i> \
  -d '{
    <span>"model"</span>: <em>"your-model"</em>,
    <span>"max_tokens"</span>: <em>1024</em>,
    <span>"messages"</span>: [
      { <span>"role"</span>: <em>"user"</em>, <span>"content"</span>: <em>"..."</em> }
    ]
  }'</code></pre>
          </div>
          <div class="response-block">
            <p class="label">RESPONSE</p>
            <pre><code>{
  <span>"content"</span>: [{ <span>"type"</span>: <em>"text"</em>, <span>"text"</span>: <em>"{{ activeApiTab }} message routed."</em> }],
  <span>"usage"</span>: { <span>"input_tokens"</span>: 11, <span>"output_tokens"</span>: 18 }
}</code></pre>
          </div>
          <div class="api-footer"><span>156 MS　·　29 TOKENS　·　COST $0.00007</span><span>STREAM　·　SSE</span></div>
        </div>
      </section>

      <section class="feature-grid section-shell">
        <article v-for="feature in features" :key="feature.title" class="feature-card">
          <span class="feature-icon" :class="feature.color">{{ feature.icon }}</span>
          <h3>{{ feature.title }}</h3>
          <p>{{ feature.description }}</p>
        </article>
      </section>

      <section id="models" class="models section-shell">
        <p class="section-kicker">已支持的 AI 模型</p>
        <p class="section-note">一个 API，多种选择</p>
        <div class="model-list">
          <span v-for="model in models" :key="model.name" class="model-pill"><i :class="model.color">{{ model.mark }}</i>{{ model.name }} <b>已支持</b></span>
          <span class="model-pill muted"><i>＋</i> 更多 <small>即将推出</small></span>
        </div>
      </section>

      <section id="metrics" class="metrics section-shell">
        <div v-for="metric in metrics" :key="metric.label" class="metric"><b>{{ metric.value }}</b><span>{{ metric.label }}</span></div>
      </section>

      <section class="capabilities section-shell">
        <p class="section-note">核心能力</p>
        <h2>为开发者打造，面向规模化设计</h2>
        <div class="capability-board">
          <article class="capability wide">
            <span>01</span><h3>极速响应</h3><p>优化的网络架构，确保毫秒级响应时间</p>
            <div class="provider-cloud"><b>OpenAI</b><b>Claude</b><b>Gemini</b><b>DeepSeek</b><b>Qwen</b><b>Llama</b></div>
          </article>
          <article class="capability shield"><span>02</span><h3>安全可靠</h3><p>企业级安全防护，完善的权限管理体系</p><i>♢</i></article>
          <article class="capability"><span>03</span><h3>全球覆盖</h3><p>多地域部署，稳定的全球访问体验</p><ol><li>负载均衡</li><li>故障切换</li><li>成本追踪</li></ol></article>
          <article class="capability"><span>04</span><h3>开发者友好</h3><p>兼容主流 API 协议，快速集成现有工作流</p><div class="mini-tags"><b>API</b><b>SDK</b><b>CLI</b><b>Docs</b></div></article>
        </div>
        <div class="benefit-row"><span>◌ <b>高性能</b><small>自动负载均衡</small></span><span>♧ <b>透明计费</b><small>实时用量监控</small></span><span>♧ <b>团队协作</b><small>灵活的权限分配</small></span><span>♧ <b>开发友好</b><small>自托管，可扩展</small></span></div>
      </section>

      <section id="quick-start" class="quick-start section-shell">
        <p class="section-note">如何开始</p><h2>三步快速接入</h2>
        <div class="steps">
          <article v-for="step in steps" :key="step.number"><span class="step-number">{{ step.number }}</span><i>{{ step.icon }}</i><h3>{{ step.title }}</h3><p>{{ step.description }}</p></article>
        </div>
      </section>

      <section class="cta section-shell">
        <div><h2>准备简化你的 AI 集成了吗？</h2><p>立即部署你的网关，开始路由请求，掌控你的 AI 服务。</p></div>
        <div class="cta-actions"><router-link :to="isAuthenticated ? dashboardPath : '/login'" class="primary-button">进入控制台　→</router-link><a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="secondary-button">查看定价</a></div>
      </section>
    </main>

    <footer class="site-footer section-shell"><div class="brand footer-brand"><img src="/starx-logo-transparent.png" alt="star-X" class="brand-logo" /></div><span>强大的 AI API 管理平台。</span><small>© {{ currentYear }} star-X. 保留所有权利。</small></footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import { sanitizeUrl } from '@/utils/url'

const authStore = useAuthStore()
const appStore = useAppStore()
const isDark = ref(false)
const activeApiTab = ref('Chat')
const apiTabs = ['Chat', 'Responses', 'Claude', 'Gemini']
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const isHomeContentUrl = computed(() => /^https?:\/\//.test(homeContent.value.trim()))
const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const currentYear = computed(() => new Date().getFullYear())
const features = [
  { icon: '▣', color: 'blue', title: '自动接入', description: '在线一个 API 密钥，即可调用所有已接入的 AI 模型，无需分别申请。' },
  { icon: '⌘', color: 'indigo', title: '非常稳', description: '智能调度多个上游账号，自动切换和负载均衡，告别频繁报错。' },
  { icon: '▤', color: 'violet', title: '按量付费', description: '按实际使用量计费，支持设置配额上限，团队用量一目了然。' },
]
const models = [
  { name: 'Claude', mark: 'C', color: 'orange' }, { name: 'GPT', mark: 'G', color: 'green' }, { name: 'Gemini', mark: 'G', color: 'blue' }, { name: 'Antigravity', mark: 'A', color: 'pink' },
]
const metrics = [{ value: '50+', label: '上游服务集成' }, { value: '100+', label: '模型计费支持' }, { value: '50+', label: '兼容 API 接口' }, { value: '10+', label: '调度控制策略' }]
const steps = [
  { number: '1', icon: '⚙', title: '配置接入', description: '添加 API Key，设置渠道和访问权限' },
  { number: '2', icon: 'ϟ', title: '选择模型', description: '选择并连接 OpenAI、Claude、Gemini 等 API 路由' },
  { number: '3', icon: '⌁', title: '监控使用', description: '实时监控使用量、成本和性能指标' },
]
function toggleTheme() { isDark.value = !isDark.value; document.documentElement.classList.toggle('dark', isDark.value); localStorage.setItem('theme', isDark.value ? 'dark' : 'light') }
onMounted(() => { isDark.value = localStorage.getItem('theme') === 'dark'; document.documentElement.classList.toggle('dark', isDark.value); authStore.checkAuth(); if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings() })
</script>

<style scoped>
.starx-home { --ink:#151824; --muted:#727889; --line:#e9edf4; --blue:#1686ef; --blue2:#2877ee; min-height:100vh; color:var(--ink); background:radial-gradient(circle at 17% 10%,#f3f8ff 0,transparent 22%),#fff; font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif; }
.is-dark { --ink:#f6f8fc; --muted:#aeb7c7; --line:#273144; background:#101622; }
.section-shell,.nav-shell { width:min(1180px,calc(100% - 48px)); margin-inline:auto; }.site-header { padding:24px 0 10px; }.nav-shell { display:flex;align-items:center;justify-content:space-between;gap:24px; }.brand { display:inline-flex;align-items:center;text-decoration:none;overflow:hidden; }.brand-logo{width:96px;height:42px;object-fit:cover;object-position:center}.nav-links,.nav-actions{display:flex;align-items:center;gap:27px}.nav-links a{position:relative;padding:11px 0;color:#515767;text-decoration:none;font-size:14px}.nav-links a:hover,.nav-links .active{color:#161b27}.nav-links .active:after{content:"";position:absolute;bottom:0;left:0;width:100%;height:2px;background:#3187ee}.nav-actions{gap:12px}.icon-button{border:0;background:transparent;color:var(--ink);padding:7px;cursor:pointer}.login-button,.primary-button{border:0;border-radius:11px;background:linear-gradient(120deg,#0b91ef,#276eea);color:white;text-decoration:none;font-weight:600;box-shadow:0 8px 18px #1986ee35}.login-button{padding:9px 18px;font-size:13px}.hero{display:grid;grid-template-columns:.9fr 1.25fr;align-items:center;gap:80px;padding-top:75px;padding-bottom:65px}.eyebrow{display:inline-flex;align-items:center;gap:7px;border:1px solid #cfe2fa;border-radius:99px;padding:7px 12px;color:#2589ec;background:#f5faff;font-size:12px}.eyebrow span{width:6px;height:6px;border-radius:50%;background:#1b91ed}.hero-wordmark{display:flex;align-items:baseline;margin:24px 0 7px;font-family:Arial Black,Inter,"PingFang SC",sans-serif;font-size:clamp(68px,6.2vw,104px);font-style:italic;font-weight:900;letter-spacing:-.06em;line-height:.82;transform:skewX(-8deg);transform-origin:left center}.hero-wordmark span{color:#07090e}.hero-wordmark b{display:inline-block;margin-left:.13em;padding-right:.08em;background:linear-gradient(135deg,#0fa9fa 12%,#2679ed 50%,#832be9 91%);background-clip:text;-webkit-background-clip:text;color:transparent;font-style:normal;letter-spacing:0}.hero-copy h2{margin:28px 0 19px;font-size:29px;letter-spacing:1px}.hero-copy h2 strong{color:#3a76ee}.hero-copy>p{margin:0;line-height:1.9;color:var(--muted);font-size:15px}.hero-actions{margin-top:34px}.primary-button{display:inline-flex;align-items:center;gap:13px;padding:14px 24px;font-size:14px}.primary-button b{font-size:18px}.trust-list{display:flex;gap:26px;margin-top:25px;color:#5b6270;font-size:13px}.api-panel{overflow:hidden;border:1px solid #edf0f5;border-radius:22px;background:#fff;box-shadow:0 17px 30px #163e6920}.is-dark .api-panel{background:#151c28;border-color:#2d374b}.api-tabs{display:flex;align-items:center;gap:30px;padding:0 22px;border-bottom:1px solid var(--line)}.api-tabs button{border:0;border-bottom:2px solid transparent;background:transparent;padding:16px 0 13px;color:#757b89;cursor:pointer;font-size:13px}.api-tabs button.active{color:#168bef;border-color:#168bef}.api-status{margin-left:auto;color:#7f8794;font:11px ui-monospace,monospace}.api-status i{display:inline-block;width:7px;height:7px;border-radius:50%;margin-right:7px;background:#10bf88}.endpoint{padding:13px 22px;border-bottom:1px solid var(--line);font:600 13px ui-monospace,monospace}.endpoint span{margin-right:12px;border-radius:4px;padding:4px 6px;background:#e5faf4;color:#1da985;font-size:10px}.code-block,.response-block{padding:17px 22px}.response-block{border-top:1px solid var(--line)}.label{margin:0 0 9px;color:#9ca3af;font:700 10px ui-monospace,monospace;letter-spacing:1px}.api-panel pre{margin:0;white-space:pre-wrap;color:#447297;font:12px/1.55 ui-monospace,SFMono-Regular,Consolas,monospace}.api-panel code b{color:#16ab8b}.api-panel code strong{color:#ee8053}.api-panel code i{color:#d58724;font-style:normal}.api-panel code span{color:#3f84bc}.api-panel code em{color:#dd9458;font-style:normal}.api-footer{display:flex;justify-content:space-between;padding:11px 22px;background:#fbfcfe;border-top:1px solid var(--line);color:#8d95a1;font:10px ui-monospace,monospace}.is-dark .api-footer{background:#111722}.feature-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:18px;padding-bottom:48px}.feature-card{border:1px solid var(--line);border-radius:13px;padding:26px 24px;background:color-mix(in srgb,#fff 94%,transparent);transition:.2s}.feature-card:hover{transform:translateY(-3px);box-shadow:0 12px 26px #3971ad13}.feature-icon{display:grid;place-items:center;width:35px;height:35px;border-radius:8px;color:#fff;font-size:18px}.feature-icon.blue{background:#1182ed}.feature-icon.indigo{background:#4f39bd}.feature-icon.violet{background:#7137de}.feature-card h3{margin:18px 0 8px;font-size:17px}.feature-card p{margin:0;color:var(--muted);line-height:1.75;font-size:13px}.models{text-align:center;padding:3px 0 48px}.section-kicker{margin:0;font-size:22px;font-weight:800}.section-note{margin:7px 0 0;color:#9299a7;font-size:13px}.model-list{display:flex;flex-wrap:wrap;justify-content:center;gap:12px;margin-top:20px}.model-pill{display:flex;align-items:center;gap:8px;border:1px solid var(--line);border-radius:10px;padding:9px 14px;background:#fff;font-size:13px;box-shadow:0 3px 8px #133b6510}.is-dark .model-pill{background:#18202d}.model-pill i{display:grid;place-items:center;width:20px;height:20px;border-radius:5px;color:white;font-size:12px;font-style:normal;font-weight:700}.model-pill .orange{background:#f78a14}.model-pill .green{background:#0eb673}.model-pill .blue{background:#1685e8}.model-pill .pink{background:#ee3c68}.model-pill b{border-radius:4px;background:#edf7ff;padding:3px 5px;color:#2286e8;font-size:10px}.model-pill.muted{color:#606775}.model-pill.muted i{background:#edf0f4;color:#5d6470}.model-pill small{color:#a1a7b3}.metrics{display:grid;grid-template-columns:repeat(4,1fr);margin-bottom:56px;padding:21px 34px;border:1px solid #edf1f6;border-radius:17px;background:#f8fbff}.metric{display:grid;gap:5px;text-align:center;border-right:1px solid #e7edf5}.metric:last-child{border:0}.metric b{color:#1682e8;font-size:25px}.metric span{color:#6d7483;font-size:12px}.capabilities{text-align:center;padding-bottom:74px}.capabilities h2,.quick-start h2{margin:7px 0 24px;font-size:25px}.capability-board{display:grid;grid-template-columns:1fr 1fr;border:1px solid var(--line);border-radius:14px;overflow:hidden;text-align:left}.capability{position:relative;min-height:183px;padding:24px;border-right:1px solid var(--line);border-top:1px solid var(--line)}.capability:nth-child(1),.capability:nth-child(2){border-top:0}.capability:nth-child(even){border-right:0}.capability span{color:#8e96a3;font-size:11px}.capability h3{margin:10px 0 5px;font-size:15px}.capability p{margin:0;color:var(--muted);font-size:12px}.provider-cloud{display:flex;flex-wrap:wrap;gap:20px;margin-top:22px;color:#667080;font-size:11px}.shield i{position:absolute;right:27%;bottom:22px;display:grid;place-items:center;width:48px;height:48px;border-radius:50%;background:#e8fbf5;color:#35ba94;font-size:25px}.capability ol{position:absolute;right:25px;bottom:9px;margin:0;color:#87909d;font-size:11px;line-height:1.8}.mini-tags{display:flex;gap:8px;margin-top:24px}.mini-tags b{border-radius:5px;background:#f5f7fa;padding:5px 8px;color:#656d79;font-size:10px}.benefit-row{display:grid;grid-template-columns:repeat(4,1fr);padding:19px;border-bottom:1px solid var(--line);text-align:left}.benefit-row span{display:grid;grid-template-columns:23px 1fr;column-gap:7px;color:#6b7480;font-size:18px}.benefit-row b{font-size:13px}.benefit-row small{grid-column:2;color:#9098a5;font-size:11px}.quick-start{text-align:center;padding-bottom:72px}.steps{display:grid;grid-template-columns:repeat(3,1fr);gap:70px;margin-top:31px}.steps article{position:relative}.steps article:not(:last-child):after{content:"";position:absolute;top:30px;left:67%;width:66%;border-top:1px dashed #cfd7e3}.step-number{position:absolute;top:-5px;left:calc(50% + 11px);display:grid;place-items:center;width:18px;height:18px;border-radius:50%;background:#111;color:#fff;font-size:10px}.steps i{display:grid;place-items:center;width:52px;height:52px;margin:auto;border-radius:50%;background:#f7f9fc;color:#4c5461;font-size:22px;font-style:normal}.steps h3{margin:17px 0 8px;font-size:14px}.steps p{margin:auto;max-width:210px;color:var(--muted);font-size:12px;line-height:1.7}.cta{display:flex;align-items:center;justify-content:space-between;gap:25px;margin-bottom:24px;padding:34px 48px;border-radius:15px;background:linear-gradient(100deg,#c6e1ff,#eed1ff);text-align:center}.cta h2{margin:0 0 7px;font-size:22px}.cta p{margin:0;color:#4d5666;font-size:13px}.cta-actions{display:flex;gap:12px;flex-shrink:0}.secondary-button{padding:14px 22px;border-radius:11px;background:#fff;color:#4d5664;text-decoration:none;font-size:13px;font-weight:600;box-shadow:0 3px 10px #6d49a022}.site-footer{display:flex;align-items:center;gap:17px;padding:8px 0 28px;color:#7d8592;font-size:12px}.footer-brand{font-size:24px}.footer-brand .brand-logo{width:83px;height:37px}.site-footer small{margin-left:auto;color:#a3a9b3}
.is-dark .brand-logo{filter:invert(1)}.is-dark .hero-wordmark span{color:#f6f8fc}
.is-dark .nav-links a{color:#93a0b5}.is-dark .nav-links a:hover,.is-dark .nav-links .active{color:#f4f7fc}
.is-dark .eyebrow{border-color:#28547d;background:#14263a;color:#69b9ff}.is-dark .trust-list{color:#9eacc0}
.is-dark .api-tabs button{color:#a7b2c4}.is-dark .api-tabs button.active{color:#58aeff}.is-dark .api-status{color:#a7b3c5}.is-dark .endpoint{color:#edf5ff}.is-dark .endpoint span{background:#173b36;color:#55d7ad}
.is-dark .api-panel pre{color:#91c8ee}.is-dark .api-panel code span{color:#69b6f5}.is-dark .api-panel code em{color:#f0af75}.is-dark .api-footer{color:#9eacc0}
.is-dark .feature-card,.is-dark .model-pill{background:#18212f;border-color:#2d3a4f;box-shadow:0 12px 26px #02060d44}.is-dark .feature-card:hover{box-shadow:0 18px 34px #02060d88}.is-dark .feature-card h3{color:#f4f7fc}
.is-dark .model-pill b{background:#173653;color:#73c0ff}.is-dark .model-pill.muted{color:#b4becd}.is-dark .model-pill.muted i{background:#2a3444;color:#c7d0dd}
.is-dark .metrics{border-color:#2d3a4f;background:#151e2b}.is-dark .metric{border-color:#2d3a4f}.is-dark .metric span{color:#aeb9c9}
.is-dark .capability-board{background:#151e2b;border-color:#2d3a4f}.is-dark .capability{border-color:#2d3a4f}.is-dark .capability h3,.is-dark .capabilities h2,.is-dark .quick-start h2{color:#f5f8fd}.is-dark .provider-cloud{color:#aeb9c9}.is-dark .shield i{background:#173a39;color:#5de3be}.is-dark .capability ol{color:#abb7c8}.is-dark .mini-tags b{background:#283344;color:#d6deea}
.is-dark .benefit-row{border-color:#2d3a4f}.is-dark .benefit-row span{color:#c6d0de}.is-dark .benefit-row small{color:#95a3b7}.is-dark .steps i{background:#1b2635;color:#bfcce0}.is-dark .step-number{background:#4b9cff;color:#07111d}
.is-dark .cta{background:linear-gradient(110deg,#173757,#2b1d4a);border:1px solid #3b3f72}.is-dark .cta h2{color:#f7f9fd}.is-dark .cta p{color:#c1cce0}.is-dark .secondary-button{background:#263347;color:#e9f0fb;box-shadow:none}
.is-dark .site-footer{color:#a8b5c7}.is-dark .site-footer small{color:#8594aa}
@media (max-width:900px){.nav-links{display:none}.hero{grid-template-columns:1fr;gap:45px;padding-top:54px}.hero-copy{text-align:center}.hero-wordmark{justify-content:center;margin-inline:auto}.trust-list{justify-content:center}.api-panel{max-width:650px;margin:auto}.feature-grid{grid-template-columns:1fr}.metrics{grid-template-columns:1fr 1fr;gap:22px}.metric:nth-child(2){border-right:0}.capability-board{grid-template-columns:1fr}.capability,.capability:nth-child(odd){border-right:0;border-top:1px solid var(--line)}.capability:first-child{border-top:0}.benefit-row{grid-template-columns:1fr 1fr;gap:18px}.steps{gap:35px}.cta{flex-direction:column;padding:31px 24px}.site-footer{flex-wrap:wrap}.site-footer small{margin-left:0;width:100%}}
@media (max-width:560px){.section-shell,.nav-shell{width:min(100% - 32px,1180px)}.site-header{padding-top:16px}.locale-switcher{display:none}.hero-brand{font-size:54px}.hero-copy h2{font-size:24px}.hero-copy>p br{display:none}.trust-list{gap:12px;font-size:11px}.api-tabs{gap:14px;padding:0 14px}.api-tabs button{font-size:11px}.api-status{font-size:9px}.endpoint,.code-block,.response-block{padding-left:14px;padding-right:14px}.api-panel pre{font-size:10px}.api-footer{padding:10px 14px;font-size:8px}.model-list{gap:7px}.metrics{padding:18px}.capabilities h2,.quick-start h2{font-size:21px}.benefit-row{display:none}.steps{grid-template-columns:1fr;gap:34px}.steps article:not(:last-child):after{display:none}.cta h2{font-size:19px}.site-footer{justify-content:center;text-align:center}.site-footer small{width:auto}.nav-actions{gap:3px}}
</style>
