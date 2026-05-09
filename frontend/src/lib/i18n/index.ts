import { writable, get } from 'svelte/store';
import { translations } from './translations';

export type LanguageCode = keyof typeof translations;

const DEFAULT_LANG: LanguageCode = 'en';

export const language = writable<LanguageCode>(DEFAULT_LANG);

export function initLanguage(): void {
	const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('lang') : null;
	if (stored && stored in translations) {
		language.set(stored as LanguageCode);
		return;
	}
	const navigatorLanguages =
		typeof navigator !== 'undefined'
			? [navigator.language, ...(navigator.languages ?? [])].filter(Boolean)
			: [];
	const hasFrenchMatch = navigatorLanguages.some((lang) => lang.toLowerCase().startsWith('fr'));
	language.set(hasFrenchMatch ? 'fr' : DEFAULT_LANG);
}

export function setLanguage(next: LanguageCode): void {
	if (!(next in translations)) return;
	language.set(next);
	if (typeof localStorage !== 'undefined') {
		localStorage.setItem('lang', next);
	}
}

export function t(key: string, params?: Record<string, string | number>): string {
	const lang = get(language);
	const entry = translations[lang] ?? translations[DEFAULT_LANG];
	const raw = entry?.[key as keyof typeof entry] ?? key;
	if (!params) return raw as string;
	return Object.entries(params).reduce((acc, [paramKey, value]) => {
		return acc.replaceAll(`{${paramKey}}`, String(value));
	}, raw as string);
}
