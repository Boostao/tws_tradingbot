#!/usr/bin/env python3
"""
Main entry point for the TWS Trading Bot.

Run this script to start the Streamlit UI.
"""

import streamlit as st
from src.ui.dashboard import display_dashboard

def main():
    display_dashboard()

if __name__ == "__main__":
    main()