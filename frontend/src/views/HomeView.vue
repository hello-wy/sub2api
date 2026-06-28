<template>
  <div class="relative flex h-screen max-h-screen w-full flex-col justify-between overflow-hidden bg-gradient-to-br from-gray-50 via-blue-50/30 to-slate-100 text-gray-800 font-sans select-none box-border">
    
    <!-- Ambient Radial Mesh Background -->
    <div class="pointer-events-none absolute inset-0 z-0 overflow-hidden">
      <div class="absolute -top-40 -right-40 w-[600px] h-[600px] rounded-full bg-blue-400/15 blur-[120px]"></div>
      <div class="absolute -bottom-40 -left-40 w-[600px] h-[600px] rounded-full bg-indigo-400/15 blur-[120px]"></div>
      <div class="absolute top-1/3 left-1/2 -translate-x-1/2 w-[800px] h-[400px] rounded-full bg-cyan-300/20 blur-[140px]"></div>
    </div>

    <!-- Header Section (Aligned with main content left/right margin) -->
    <header class="relative z-30 max-w-[1440px] mx-auto w-full px-6 lg:px-12 py-4 bg-transparent flex items-center justify-between">
      <!-- Brand Logo only -->
      <div class="flex items-center">
        <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md bg-white p-1 border border-gray-100 flex items-center justify-center">
          <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
        </div>
      </div>

      <!-- Navigation links -->
      <nav class="hidden md:flex items-center gap-8 text-sm font-medium text-gray-700">
        <a href="#" class="flex items-center gap-1 hover:text-blue-600 transition-colors">
          {{ t('home.nav.products') }}
          <svg class="w-3.5 h-3.5 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
        </a>
        <a href="#" class="hover:text-blue-600 transition-colors">{{ t('home.nav.pricing') }}</a>
        <a v-if="docUrl" :href="docUrl" target="_blank" class="hover:text-blue-600 transition-colors">{{ t('home.nav.docs') }}</a>
        <a v-else href="#" class="hover:text-blue-600 transition-colors">{{ t('home.nav.docs') }}</a>
        <a href="#" class="hover:text-blue-600 transition-colors">{{ t('home.nav.help') }}</a>
        <a href="#" class="hover:text-blue-600 transition-colors">{{ t('home.nav.about') }}</a>
      </nav>

      <!-- Right Actions (Aligned with main right margin) -->
      <div class="flex items-center gap-3.5">
        <!-- Working Language Selector Dropdown -->
        <div class="relative" ref="langDropdownRef">
          <button
            @click="toggleLangDropdown"
            class="flex items-center gap-1.5 px-4 py-2 rounded-full bg-white border border-gray-200 text-sm font-medium text-gray-700 hover:bg-gray-50 shadow-sm transition-all cursor-pointer"
          >
            <span class="text-base">{{ currentLocale.flag }}</span>
            <span>{{ currentLocale.code.toUpperCase() }}</span>
            <svg class="w-3.5 h-3.5 text-gray-400 transition-transform duration-200" :class="{ 'rotate-180': isLangOpen }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
          </button>
          
          <div v-if="isLangOpen" class="absolute right-0 mt-2 w-32 rounded-xl bg-white border border-gray-100 shadow-xl py-1 z-50 overflow-hidden">
            <button
              v-for="item in availableLocales"
              :key="item.code"
              @click="selectLocale(item.code)"
              class="flex items-center w-full px-3 py-2 text-xs font-medium text-gray-700 hover:bg-blue-50 hover:text-blue-600 transition-colors gap-2 cursor-pointer"
              :class="{ 'bg-blue-50/70 text-blue-600 font-bold': item.code === currentLocaleCode }"
            >
              <span>{{ item.flag }}</span>
              <span>{{ item.name }}</span>
            </button>
          </div>
        </div>

        <!-- Auth / Dashboard Buttons -->
        <router-link
          v-if="!isAuthenticated"
          to="/login"
          class="px-5 py-2 rounded-full bg-white border border-gray-300 text-sm font-semibold text-gray-700 hover:bg-gray-50 shadow-sm transition-all"
        >
          {{ t('home.login') }}
        </router-link>

        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="flex items-center gap-1.5 px-6 py-2 rounded-full bg-blue-600 hover:bg-blue-700 text-white text-sm font-semibold shadow-md shadow-blue-500/25 transition-all"
        >
          <span>{{ t('home.goToDashboard') }}</span>
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
        </router-link>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="relative z-10 flex-1 flex flex-col justify-between px-6 lg:px-12 py-2 max-w-[1440px] mx-auto w-full overflow-hidden">
      
      <!-- Upper Hero Row -->
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 items-center my-auto">
        
        <!-- Left Hero Content -->
        <div class="lg:col-span-5 flex flex-col justify-center pr-2">
          <!-- Serif Big Title -->
          <h1 class="text-5xl lg:text-6xl font-serif font-bold text-[#1E3A8A] tracking-tight mb-2 leading-tight">
            {{ siteName || 'SolidAPI' }}
          </h1>

          <!-- Gold Bronze Subtitle -->
          <h2 class="text-2xl lg:text-3xl font-serif font-bold text-[#9A6A38] tracking-wide mb-4">
            {{ t('home.heroTitleSubtitle') }}
          </h2>

          <!-- Descriptions -->
          <p class="text-gray-600 text-sm lg:text-base leading-relaxed font-normal mb-1">
            {{ t('home.heroDesc1') }}
          </p>
          <p class="text-gray-600 text-sm lg:text-base leading-relaxed font-normal mb-6">
            {{ t('home.heroDesc2') }}
          </p>

          <!-- Main Theme-Colored CTA Button -->
          <div class="mb-6">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="inline-flex items-center gap-3 px-8 py-3.5 rounded-2xl bg-blue-600 hover:bg-blue-700 text-white text-base font-semibold shadow-lg shadow-blue-500/30 transition-all duration-200 transform hover:-translate-y-0.5"
            >
              <span>{{ t('home.goToDashboard') }}</span>
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>
            </router-link>
          </div>

          <!-- Single Unified Feature Card with Vertical Dividers (Text strictly enclosed within card with zero overflow) -->
          <div class="inline-flex flex-wrap sm:flex-nowrap items-center p-3 sm:p-3.5 rounded-2xl bg-white border border-gray-100/90 shadow-sm hover:shadow-md transition-shadow max-w-full">
            <!-- Item 1 -->
            <div class="flex items-center gap-2.5 py-0.5 px-1.5 shrink-0">
              <div class="text-blue-600 shrink-0 flex items-center justify-center">
                <svg class="w-6 h-6" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
              </div>
              <div class="flex flex-col shrink-0">
                <span class="text-xs font-bold text-gray-900 whitespace-nowrap">{{ t('home.pills.subApi') }}</span>
                <span class="text-[11px] text-gray-500 whitespace-nowrap mt-0.5">{{ t('home.pills.subApiDesc') }}</span>
              </div>
            </div>

            <!-- Vertical Divider 1 -->
            <div class="hidden sm:block h-6 w-[1px] bg-gray-200/80 mx-2.5 shrink-0"></div>

            <!-- Item 2 -->
            <div class="flex items-center gap-2.5 py-0.5 px-1.5 shrink-0">
              <div class="text-blue-600 shrink-0 flex items-center justify-center">
                <svg class="w-6 h-6" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
              </div>
              <div class="flex flex-col shrink-0">
                <span class="text-xs font-bold text-gray-900 whitespace-nowrap">{{ t('home.pills.sticky') }}</span>
                <span class="text-[11px] text-gray-500 whitespace-nowrap mt-0.5">{{ t('home.pills.stickyDesc') }}</span>
              </div>
            </div>

            <!-- Vertical Divider 2 -->
            <div class="hidden sm:block h-6 w-[1px] bg-gray-200/80 mx-2.5 shrink-0"></div>

            <!-- Item 3 -->
            <div class="flex items-center gap-2.5 py-0.5 px-1.5 shrink-0">
              <div class="text-blue-600 shrink-0 flex items-center justify-center">
                <svg class="w-6 h-6" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>
              </div>
              <div class="flex flex-col shrink-0">
                <span class="text-xs font-bold text-gray-900 whitespace-nowrap">{{ t('home.pills.billing') }}</span>
                <span class="text-[11px] text-gray-500 whitespace-nowrap mt-0.5">{{ t('home.pills.billingDesc') }}</span>
              </div>
            </div>
          </div>

        </div>

        <!-- Right Console Preview Window with 3D Tilt Card and Short Wide Pedestal -->
        <div class="lg:col-span-7 flex flex-col items-center justify-center">
          
          <div class="relative w-full max-w-2xl flex flex-col items-center group/console">
            
            <!-- 3D Tilt Window Container -->
            <div class="w-full rounded-2xl bg-[#0B132B] text-white p-5 shadow-[0_25px_60px_-15px_rgba(15,23,42,0.6)] border border-slate-700/70 relative z-10 transform lg:[transform:perspective(1000px)_rotateX(4deg)_rotateY(-4deg)] lg:group-hover/console:[transform:perspective(1000px)_rotateX(0deg)_rotateY(0deg)_scale(1.02)] transition-all duration-500 ease-out">
              
              <!-- Console Top Header -->
              <div class="flex items-center justify-between pb-3 mb-3 border-b border-slate-800 text-xs text-slate-400">
                <div class="flex items-center gap-2">
                  <div class="h-5 w-5 rounded-md overflow-hidden bg-white p-0.5 flex items-center justify-center shadow-sm">
                    <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
                  </div>
                  <span class="font-semibold text-slate-200 text-sm">{{ siteName || 'SolidAPI' }} {{ t('home.console.title') }}</span>
                </div>
                <div class="flex items-center gap-4 text-xs">
                  <span class="flex items-center gap-1.5">
                    <span class="h-2 w-2 rounded-full bg-emerald-500 animate-pulse"></span>
                    {{ t('home.console.serviceNormal') }}
                  </span>
                  <span class="flex items-center gap-1 text-slate-400">
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                    {{ t('home.console.latency') }} 12ms
                  </span>
                </div>
              </div>

              <!-- Architecture Flow Area -->
              <div class="relative grid grid-cols-12 gap-3 items-center py-2">
                
                <!-- Left Column: Upstream Suppliers (Cards 1:1 matching Image 1) -->
                <div class="col-span-4 flex flex-col gap-2 z-10 py-1 pr-1">
                  <span class="text-xs font-medium text-slate-400 mb-0.5">{{ t('home.console.upstream') }}</span>
                  
                  <!-- Supplier 1: OpenAI -->
                  <div class="flex items-center justify-between p-2 rounded-xl bg-slate-900/90 border border-slate-800 text-xs shadow-sm hover:border-slate-700 transition-colors">
                    <div class="flex items-center gap-2 min-w-0">
                      <div class="h-6 w-6 rounded-full bg-black flex items-center justify-center text-white shrink-0">
                        <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.09 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729z"/></svg>
                      </div>
                      <div class="truncate">
                        <div class="font-semibold text-slate-200 text-xs">OpenAI</div>
                        <div class="text-[9px] text-slate-400">GPT-4o</div>
                      </div>
                    </div>
                    <span class="inline-flex items-center gap-1 text-[9px] text-emerald-400 shrink-0">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>{{ t('home.console.available') }}
                    </span>
                  </div>

                  <!-- Supplier 2: Anthropic -->
                  <div class="flex items-center justify-between p-2 rounded-xl bg-slate-900/90 border border-slate-800 text-xs shadow-sm hover:border-slate-700 transition-colors">
                    <div class="flex items-center gap-2 min-w-0">
                      <div class="h-6 w-6 rounded-md bg-amber-900/40 border border-amber-600/50 flex items-center justify-center text-xs font-bold text-amber-300 shrink-0">AI</div>
                      <div class="truncate">
                        <div class="font-semibold text-slate-200 text-xs">Anthropic</div>
                        <div class="text-[9px] text-slate-400">Claude 3.5 Sonnet</div>
                      </div>
                    </div>
                    <span class="inline-flex items-center gap-1 text-[9px] text-emerald-400 shrink-0">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>{{ t('home.console.available') }}
                    </span>
                  </div>

                  <!-- Supplier 3: Google Gemini -->
                  <div class="flex items-center justify-between p-2 rounded-xl bg-slate-900/90 border border-slate-800 text-xs shadow-sm hover:border-slate-700 transition-colors">
                    <div class="flex items-center gap-2 min-w-0">
                      <div class="h-6 w-6 rounded-full bg-white flex items-center justify-center shrink-0">
                        <svg class="w-4 h-4" viewBox="0 0 24 24"><path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/><path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/><path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/><path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/></svg>
                      </div>
                      <div class="truncate">
                        <div class="font-semibold text-slate-200 text-xs">Google</div>
                        <div class="text-[9px] text-slate-400">Gemini 1.5 Pro</div>
                      </div>
                    </div>
                    <span class="inline-flex items-center gap-1 text-[9px] text-emerald-400 shrink-0">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>{{ t('home.console.available') }}
                    </span>
                  </div>

                  <!-- Supplier 4: More (+) -->
                  <div class="flex items-center justify-between p-2 rounded-xl bg-slate-900/90 border border-slate-800 text-xs shadow-sm hover:border-slate-700 transition-colors">
                    <div class="flex items-center gap-2 min-w-0">
                      <div class="h-6 w-6 rounded-lg bg-blue-600/30 border border-blue-500/40 flex items-center justify-center text-blue-400 font-bold text-xs shrink-0">+</div>
                      <div class="truncate">
                        <div class="font-semibold text-slate-200 text-xs">{{ t('home.providers.more') }}</div>
                        <div class="text-[9px] text-slate-400">{{ t('home.providers.soon') }}</div>
                      </div>
                    </div>
                    <span class="inline-flex items-center gap-1 text-[9px] text-emerald-400 shrink-0">
                      <span class="h-1.5 w-1.5 rounded-full bg-emerald-400"></span>{{ t('home.console.available') }}
                    </span>
                  </div>
                </div>

                <!-- Animated SVG Flow Lines overlay (Replicating Image 1 1:1 with flowing green light) -->
                <svg class="absolute inset-0 w-full h-full pointer-events-none z-0" viewBox="0 0 500 200" fill="none">
                  <defs>
                    <linearGradient id="flow-grad-green" x1="0%" y1="0%" x2="100%" y2="0%">
                      <stop offset="0%" stop-color="#10B981" stop-opacity="0.4"/>
                      <stop offset="50%" stop-color="#34D399" stop-opacity="1"/>
                      <stop offset="100%" stop-color="#059669" stop-opacity="0.8"/>
                    </linearGradient>
                  </defs>

                  <!-- Base Green Smooth Curved Lines -->
                  <path d="M 165 36 C 190 36, 190 96, 205 96" stroke="#10B981" stroke-width="1.5" opacity="0.6"/>
                  <path d="M 165 76 C 185 76, 190 96, 205 96" stroke="#10B981" stroke-width="1.5" opacity="0.8"/>
                  <path d="M 165 116 C 185 116, 190 96, 205 96" stroke="#10B981" stroke-width="1.5" opacity="0.8"/>
                  <path d="M 165 156 C 190 156, 190 96, 205 96" stroke="#10B981" stroke-width="1.5" opacity="0.6"/>

                  <!-- Flowing Light Particle overlay lines -->
                  <path d="M 165 36 C 190 36, 190 96, 205 96" stroke="url(#flow-grad-green)" stroke-width="2.5" class="animate-flow-line" />
                  <path d="M 165 76 C 185 76, 190 96, 205 96" stroke="url(#flow-grad-green)" stroke-width="2.5" class="animate-flow-line" />
                  <path d="M 165 116 C 185 116, 190 96, 205 96" stroke="url(#flow-grad-green)" stroke-width="2.5" class="animate-flow-line" />
                  <path d="M 165 156 C 190 156, 190 96, 205 96" stroke="url(#flow-grad-green)" stroke-width="2.5" class="animate-flow-line" />

                  <!-- Green Connection Port Dots (Matching Image 1) -->
                  <circle cx="205" cy="96" r="4" fill="#10B981" stroke="#ECFDF5" stroke-width="1.5" />
                  <circle cx="310" cy="96" r="4" fill="#10B981" stroke="#ECFDF5" stroke-width="1.5" />

                  <!-- Right Flow Line to User Client -->
                  <path d="M 310 96 L 345 96" stroke="url(#flow-grad-green)" stroke-width="2.5" stroke-dasharray="4 4" class="animate-flow-line" />
                </svg>

                <!-- Center Column: Unified SolidAPI Hub Node -->
                <div class="col-span-4 flex flex-col items-center justify-center z-10 px-1">
                  <div class="w-full flex flex-col items-center p-4 rounded-xl bg-gradient-to-b from-slate-900 to-slate-950 border border-blue-500/50 shadow-xl shadow-blue-500/20 relative group">
                    
                    <div class="flex items-center gap-2 mb-2">
                      <!-- Unified Site Logo in Center Hub -->
                      <div class="h-7 w-7 rounded-lg overflow-hidden bg-white p-0.5 flex items-center justify-center shadow-md">
                        <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
                      </div>
                      <span class="font-bold text-base text-white tracking-wide">{{ siteName || 'SolidAPI' }}</span>
                    </div>

                    <div class="text-[10px] font-semibold text-blue-300 bg-blue-950/90 border border-blue-700/60 rounded-full px-2.5 py-0.5 mb-3 text-center whitespace-nowrap shadow-sm">
                      {{ t('home.console.hubFeature') }}
                    </div>

                    <div class="flex items-center justify-between w-full pt-2 border-t border-slate-800/90 text-xs text-slate-200 font-medium">
                      <span class="flex items-center gap-1"><svg class="w-3.5 h-3.5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg> {{ t('home.console.stable') }}</span>
                      <span class="flex items-center gap-1"><svg class="w-3.5 h-3.5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg> {{ t('home.console.fast') }}</span>
                      <span class="flex items-center gap-1"><svg class="w-3.5 h-3.5 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg> {{ t('home.console.secure') }}</span>
                    </div>
                  </div>
                </div>

                <!-- Right Column: User Representative (用户代表 / App Client) -->
                <div class="col-span-4 flex flex-col z-10">
                  <div class="flex items-center justify-between mb-1">
                    <span class="text-xs font-semibold text-blue-300 flex items-center gap-1">
                      <svg class="w-3.5 h-3.5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
                      {{ t('home.console.userClient') }}
                    </span>
                    <div class="flex items-center gap-1">
                      <span class="px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-400 font-mono text-[10px] font-bold">200 OK</span>
                      <span class="text-[10px] text-slate-400">12ms</span>
                    </div>
                  </div>

                  <div class="p-2.5 rounded-xl bg-[#070D1E] border border-slate-800 font-mono text-[10px] text-slate-300 leading-relaxed overflow-hidden shadow-inner">
                    <div class="text-slate-400">{</div>
                    <div class="pl-2"><span class="text-sky-400">"id"</span>: <span class="text-amber-300">"chatcmpl-123456"</span>,</div>
                    <div class="pl-2"><span class="text-sky-400">"object"</span>: <span class="text-amber-300">"chat.completion"</span>,</div>
                    <div class="pl-2"><span class="text-sky-400">"created"</span>: <span class="text-emerald-400">1715827200</span>,</div>
                    <div class="pl-2"><span class="text-sky-400">"model"</span>: <span class="text-amber-300">"gpt-4o"</span>,</div>
                    <div class="pl-2"><span class="text-sky-400">"choices"</span>: [ ... ]</div>
                    <div class="text-slate-400">}</div>
                  </div>
                </div>

              </div>

              <!-- Console Bottom Bar -->
              <div class="mt-2 pt-2 border-t border-slate-800/80 text-center text-xs text-slate-400 font-medium">
                {{ t('home.console.footer') }}
              </div>

            </div>

            <!-- Short and Wide 3D Cylinder Pedestal -->
            <div class="w-[94%] h-7 -mt-3.5 rounded-[50%] bg-gradient-to-b from-slate-200/80 via-slate-300/50 to-slate-400/20 shadow-[0_20px_35px_rgba(0,0,0,0.12)] border-t border-white/80 backdrop-blur-md relative z-0 flex items-center justify-center">
              <div class="w-[88%] h-3 rounded-[50%] bg-gradient-to-b from-blue-400/25 to-transparent blur-xs"></div>
            </div>

          </div>

        </div>

      </div>

      <!-- Middle Section: 3 Feature Cards -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-5 my-2">
        <!-- Feature 1 -->
        <div class="p-5 rounded-2xl bg-white/90 border border-gray-100 shadow-sm hover:shadow-md transition-all flex flex-col justify-between group relative overflow-hidden">
          <div>
            <div class="h-10 w-10 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center mb-3 group-hover:scale-105 transition-transform">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>
            </div>
            <h3 class="text-base font-bold text-gray-900 mb-1.5">{{ t('home.features.unifiedGateway') }}</h3>
            <p class="text-xs text-gray-500 leading-relaxed">
              {{ t('home.features.unifiedGatewayDesc') }}
            </p>
          </div>
          <div class="flex justify-end mt-3">
            <div class="h-6 w-6 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center text-xs">
              →
            </div>
          </div>
        </div>

        <!-- Feature 2 -->
        <div class="p-5 rounded-2xl bg-white/90 border border-gray-100 shadow-sm hover:shadow-md transition-all flex flex-col justify-between group relative overflow-hidden">
          <div>
            <div class="h-10 w-10 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center mb-3 group-hover:scale-105 transition-transform">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/></svg>
            </div>
            <h3 class="text-base font-bold text-gray-900 mb-1.5">{{ t('home.features.multiAccount') }}</h3>
            <p class="text-xs text-gray-500 leading-relaxed">
              {{ t('home.features.multiAccountDesc') }}
            </p>
          </div>
          <div class="flex justify-end mt-3">
            <div class="h-6 w-6 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center text-xs">
              →
            </div>
          </div>
        </div>

        <!-- Feature 3 -->
        <div class="p-5 rounded-2xl bg-white/90 border border-gray-100 shadow-sm hover:shadow-md transition-all flex flex-col justify-between group relative overflow-hidden">
          <div>
            <div class="h-10 w-10 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center mb-3 group-hover:scale-105 transition-transform">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h18M7 15h1m4 0h1m-7 4h12a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/></svg>
            </div>
            <h3 class="text-base font-bold text-gray-900 mb-1.5">{{ t('home.features.balanceQuota') }}</h3>
            <p class="text-xs text-gray-500 leading-relaxed">
              {{ t('home.features.balanceQuotaDesc') }}
            </p>
          </div>
          <div class="flex justify-end mt-3">
            <div class="h-6 w-6 rounded-full bg-blue-50 text-blue-600 flex items-center justify-center text-xs">
              →
            </div>
          </div>
        </div>
      </div>

      <!-- Bottom Section: Supported AI Models -->
      <div class="flex flex-col items-center pt-3 pb-16">
        <!-- Section Header with Filigree Ornaments -->
        <div class="flex items-center gap-3 mb-1">
          <svg class="w-6 h-4 text-amber-600/70" viewBox="0 0 40 20" fill="currentColor">
            <path d="M0,10 Q10,0 20,10 Q30,20 40,10 Q30,12 20,10 Q10,8 0,10 Z"/>
          </svg>
          <h3 class="text-xl font-bold text-gray-900 font-serif tracking-wide">{{ t('home.providers.title') }}</h3>
          <svg class="w-6 h-4 text-amber-600/70 transform rotate-180" viewBox="0 0 40 20" fill="currentColor">
            <path d="M0,10 Q10,0 20,10 Q30,20 40,10 Q30,12 20,10 Q10,8 0,10 Z"/>
          </svg>
        </div>
        <p class="text-xs text-gray-500 mb-4">{{ t('home.providers.description') }}</p>

        <!-- Supported Model Cards Row -->
        <div class="flex flex-wrap items-center justify-center gap-5 w-full max-w-4xl">
          <!-- Model 1: OpenAI -->
          <div class="flex items-center gap-3 px-6 py-3 rounded-xl bg-white border border-gray-100 shadow-sm hover:shadow transition-shadow">
            <div class="h-8 w-8 rounded-full bg-black flex items-center justify-center text-white shrink-0">
              <svg class="w-4.5 h-4.5" viewBox="0 0 24 24" fill="currentColor"><path d="M22.2819 9.8211a5.9847 5.9847 0 0 0-.5157-4.9108 6.0462 6.0462 0 0 0-6.5098-2.9A6.0651 6.0651 0 0 0 4.9807 4.1818a5.9847 5.9847 0 0 0-3.9977 2.9 6.0462 6.0462 0 0 0 .7427 7.09 5.98 5.98 0 0 0 .511 4.9107 6.051 6.051 0 0 0 6.5146 2.9001A5.9847 5.9847 0 0 0 13.2599 24a6.0557 6.0557 0 0 0 5.7718-4.2058 5.9894 5.9894 0 0 0 3.9977-2.9001 6.0557 6.0557 0 0 0-.7475-7.0729z"/></svg>
            </div>
            <span class="text-base font-semibold text-gray-900">OpenAI</span>
            <span class="px-2.5 py-0.5 rounded-md text-xs font-semibold bg-emerald-50 text-emerald-600 border border-emerald-200">{{ t('home.providers.supported') }}</span>
          </div>

          <!-- Model 2: Anthropic -->
          <div class="flex items-center gap-3 px-6 py-3 rounded-xl bg-white border border-gray-100 shadow-sm hover:shadow transition-shadow">
            <div class="h-8 w-8 rounded-lg bg-[#CC785C]/15 flex items-center justify-center text-[#CC785C] font-bold shrink-0">
              <svg class="w-4.5 h-4.5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M17.3 3H6.7L1 21h4.4l1.4-4.5h10.4l1.4 4.5h4.4L17.3 3zm-7.6 10L12 5.8 14.3 13H9.7z"/>
              </svg>
            </div>
            <span class="text-base font-semibold text-gray-900">Anthropic</span>
            <span class="px-2.5 py-0.5 rounded-md text-xs font-semibold bg-emerald-50 text-emerald-600 border border-emerald-200">{{ t('home.providers.supported') }}</span>
          </div>

          <!-- Model 3: Gemini -->
          <div class="flex items-center gap-3 px-6 py-3 rounded-xl bg-white border border-gray-100 shadow-sm hover:shadow transition-shadow">
            <div class="h-8 w-8 rounded-lg bg-blue-50 flex items-center justify-center shrink-0">
              <svg class="w-4.5 h-4.5" viewBox="0 0 24 24" fill="none">
                <path d="M12 0C12 6.627 17.373 12 24 12C17.373 12 12 17.373 12 24C12 17.373 6.627 12 0 12C6.627 12 12 6.627 12 0Z" fill="url(#gemini-grad-icon-lg4)"/>
                <defs>
                  <linearGradient id="gemini-grad-icon-lg4" x1="0" y1="0" x2="24" y2="24" gradientUnits="userSpaceOnUse">
                    <stop offset="0%" stop-color="#4285F4"/>
                    <stop offset="50%" stop-color="#9B51E0"/>
                    <stop offset="100%" stop-color="#E91E63"/>
                  </linearGradient>
                </defs>
              </svg>
            </div>
            <span class="text-base font-semibold text-gray-900">Gemini</span>
            <span class="px-2.5 py-0.5 rounded-md text-xs font-semibold bg-emerald-50 text-emerald-600 border border-emerald-200">{{ t('home.providers.supported') }}</span>
          </div>

          <!-- Model 4: More Models -->
          <div class="flex items-center gap-3 px-6 py-3 rounded-xl bg-white/70 border border-gray-200/60 shadow-sm text-gray-400">
            <div class="h-8 w-8 rounded-lg bg-blue-50 text-blue-500 flex items-center justify-center font-bold text-lg shrink-0">+</div>
            <div class="flex flex-col">
              <span class="text-sm font-semibold text-gray-700">{{ t('home.providers.more') }}</span>
              <span class="text-xs text-gray-400">{{ t('home.providers.soon') }}</span>
            </div>
          </div>
        </div>
      </div>

    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { setLocale, availableLocales } from '@/i18n'

const { t, locale } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'SolidAPI')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')

// Auth state & Routing
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

// Language Switcher dropdown state
const isLangOpen = ref(false)
const switching = ref(false)
const langDropdownRef = ref<HTMLElement | null>(null)

const currentLocaleCode = computed(() => locale.value)
const currentLocale = computed(() => availableLocales.find((l) => l.code === locale.value) || availableLocales[1])

function toggleLangDropdown() {
  isLangOpen.value = !isLangOpen.value
}

async function selectLocale(code: string) {
  if (switching.value || code === locale.value) {
    isLangOpen.value = false
    return
  }
  switching.value = true
  try {
    await setLocale(code)
    isLangOpen.value = false
  } finally {
    switching.value = false
  }
}

function handleClickOutside(event: MouseEvent) {
  if (langDropdownRef.value && !langDropdownRef.value.contains(event.target as Node)) {
    isLangOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
@keyframes flowLight {
  from {
    stroke-dashoffset: 32;
  }
  to {
    stroke-dashoffset: 0;
  }
}
.animate-flow-line {
  stroke-dasharray: 8 8;
  animation: flowLight 1.2s linear infinite;
}
</style>
