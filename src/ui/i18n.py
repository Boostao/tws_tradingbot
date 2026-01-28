"""
Simple internationalization for Streamlit.
"""

import streamlit as st

class I18n:
    def __init__(self, translations, default_locale="en", auto_detect=True):
        self.translations = translations
        self.default_locale = default_locale
        self.auto_detect = auto_detect
        
        if 'lang' not in st.session_state:
            if auto_detect:
                # For browser language detection, we could use JS, but for now default to en
                # In a real implementation, you might use st.components to get navigator.language
                st.session_state['lang'] = 'en'  # Default, could be enhanced
            else:
                st.session_state['lang'] = default_locale
    
    def t(self, key, **kwargs):
        lang = st.session_state.get('lang', self.default_locale)
        text = self.translations.get(lang, {}).get(key, key)
        return text.format(**kwargs)
    
    def language_selector(self):
        langs = list(self.translations.keys())
        current = st.session_state.get('lang', self.default_locale)
        index = langs.index(current) if current in langs else 0
        new_lang = st.selectbox(self.t("language"), langs, index=index, key="lang_selector")
        if new_lang != current:
            st.session_state['lang'] = new_lang
            st.rerun()