const fs = require('fs');
let content = fs.readFileSync('src/lib/i18n/translations.ts', 'utf8');

const enKeys = `
    manual_tws_config_title: "TWS Connection Configuration",
    manual_tws_config_p1: "Before the bot can execute any trades or read market data, it must be securely connected to Interactive Brokers' Trader Workstation (TWS) or IB Gateway. You can configure this connection directly from the bottom-left corner of the sidebar under the \"Connection\" tab.",
    manual_tws_config_host: "Host:",
    manual_tws_config_host_desc: "Usually 127.0.0.1 locally.",
    manual_tws_config_port: "Port:",
    manual_tws_config_port_desc: "By default, this is 7497 for Paper Trading and 7496 for Live Trading in TWS.",
    manual_tws_config_client_id: "Client ID:",
    manual_tws_config_client_id_desc: "An arbitrary ID (e.g. 1). Make sure it doesn't conflict with other plugins.",
    manual_tws_config_trading_mode: "Trading Mode:",
    manual_tws_config_trading_mode_desc: "Important! Ensure this matches your TWS session (Paper vs. Live) to prevent accidental real-money orders.",
    manual_tws_config_settings_title: "Important TWS Settings:",
    manual_tws_config_settings_desc: "For the bot to connect, TWS must be configured to accept API connections. Inside TWS, go to File → Global Configuration → API → Settings, and ensure \"Enable ActiveX and Socket Clients\" is checked.",
    manual_tws_config_read_docs: "Read the official IBKR TWS API Configuration Guide",

    manual_cockpit_title: "Cockpit",
    manual_cockpit_p1: "The Cockpit is your main control center. It dictates what the bot is going to trade. You will spend most of your time here assigning strategies to symbols.",
    manual_cockpit_workspaces: "Workspaces:",
    manual_cockpit_workspaces_desc: "You can create multiple workspaces (like tabs) to group different categories of symbols or different trading approaches.",
    manual_cockpit_slots: "Slots (Symbol Rows):",
    manual_cockpit_slots_desc: "Each row represents a specific market symbol (e.g., AAPL). For each symbol, you can manually assign a strategy that the bot will use.",
    manual_cockpit_strategy_act: "Strategy Activation:",
    manual_cockpit_strategy_act_desc: "The small \"play/pause\" toggle next to each strategy allows you to enable or suspend the strategy for that specific symbol. It gives you fine-grained control if you want to sit out of a market temporarily.",
    manual_cockpit_global: "Global System Toggle:",
    manual_cockpit_global_desc: "The large toggle at the top right acts as a master \"kill switch.\" If it's disabled, no trades will go through anywhere. Turn this on when you are ready to let the bot operate freely based on your assigned strategies.",

    manual_monitoring_title: "Monitoring",
    manual_monitoring_p1: "The Monitoring page provides a bird's-eye view of your account health and the bot's real-time actions.",
    manual_monitoring_top: "Top Dashboard:",
    manual_monitoring_top_desc: "This presents a summary of your account balance, day's profits and losses, and the number of trades the bot has executed today.",
    manual_monitoring_status: "System Status:",
    manual_monitoring_status_desc: "Shows whether the internal engine is catching data anomalies, and if the overall rule computation is running smoothly.",
    manual_monitoring_logs: "Activity Logs:",
    manual_monitoring_logs_desc: "Instead of rummaging through technical text files, important bot decisions (like order submissions, filled trades, or errors) will appear in a neat list here.",
    manual_monitoring_when: "When to use:",
    manual_monitoring_when_desc: "Keep this page open on a second monitor while the bot is running to passively ensure everything is ticking along nicely.",

    manual_watchlist_title: "Watchlist",
    manual_watchlist_p1: "The Watchlist acts as an address book of market symbols that the bot is actively tracking. The bot needs to load data for symbols before it can trade them.",
    manual_watchlist_manual: "Manual Entries:",
    manual_watchlist_manual_desc: "You can add symbols one-by-one safely into custom groups to keep them organized (e.g., Tech Stocks, Commodities).",
    manual_watchlist_feeds: "Feeds and Imports:",
    manual_watchlist_feeds_desc: "You can paste an external URL (such as a TradingView screener link) or upload a CSV file to automatically populate your watchlist with hundreds of tickers instantly.",
    manual_watchlist_visibility: "Visibility:",
    manual_watchlist_visibility_desc: "Symbols added here will become available to select inside your Cockpit.",

    manual_strategy_title: "Strategy Builder",
    manual_strategy_p1: "The Strategy Builder allows you to craft the actual intelligence and rules that the bot uses to make decisions.",
    manual_strategy_indicators: "Indicators:",
    manual_strategy_indicators_desc: "Define the mathematical tools (like Moving Averages or RSI) you want the strategy to be aware of.",
    manual_strategy_entry: "Entry Rules:",
    manual_strategy_entry_desc: "Define the exact conditions that must be met for the bot to buy an asset (e.g., 'When the price crosses over the Moving Average, buy').",
    manual_strategy_exit: "Exit Rules:",
    manual_strategy_exit_desc: "Define when the bot should sell an asset to take profit or cut its losses.",
    manual_strategy_validation: "Validation:",
    manual_strategy_validation_desc: "The button at the top right allows you to test your strategy's logic to make sure there are no typos or impossible conditions before deploying it to real money.",
`;

const frKeys = `
    manual_tws_config_title: "Configuration de la connexion TWS",
    manual_tws_config_p1: "Avant que le robot puisse exécuter des transactions ou lire des données de marché, il doit être connecté en toute sécurité à l'Interactive Brokers Trader Workstation (TWS) ou à la passerelle IB (IB Gateway). Vous pouvez configurer cette connexion directement dans le coin inférieur gauche de la barre latérale, sous l'onglet \"Connexion\".",
    manual_tws_config_host: "Hôte :",
    manual_tws_config_host_desc: "Généralement 127.0.0.1 en local.",
    manual_tws_config_port: "Port :",
    manual_tws_config_port_desc: "Par défaut, c'est 7497 pour le Paper Trading et 7496 pour le trading en direct dans TWS.",
    manual_tws_config_client_id: "ID client :",
    manual_tws_config_client_id_desc: "Un ID arbitraire (par exemple 1). Assurez-vous qu'il n'entre pas en conflit avec d'autres plugins.",
    manual_tws_config_trading_mode: "Mode de trading :",
    manual_tws_config_trading_mode_desc: "Important ! Assurez-vous que cela correspond à votre session TWS (Paper ou Direct) pour éviter des ordres avec de l'argent réel accidentels.",
    manual_tws_config_settings_title: "Paramètres TWS importants :",
    manual_tws_config_settings_desc: "Pour que le robot puisse se connecter, TWS doit être configuré pour accepter les connexions API. Dans TWS, allez dans Fichier → Configuration globale → API → Paramètres, et assurez-vous que \"Activer les clients ActiveX et Socket\" est coché.",
    manual_tws_config_read_docs: "Lire le guide de configuration officiel de l'API TWS d'IBKR",

    manual_cockpit_title: "Cockpit",
    manual_cockpit_p1: "Le Cockpit est votre centre de contrôle principal. Il dicte ce que le robot va trader. Vous passerez la majeure partie de votre temps ici à assigner des stratégies aux symboles.",
    manual_cockpit_workspaces: "Espaces de travail :",
    manual_cockpit_workspaces_desc: "Vous pouvez créer plusieurs espaces de travail (comme des onglets) pour regrouper différentes catégories de symboles ou différentes approches de trading.",
    manual_cockpit_slots: "Lignes (Symboles) :",
    manual_cockpit_slots_desc: "Chaque ligne représente un symbole de marché spécifique (par exemple, AAPL). Pour chaque symbole, vous pouvez attribuer manuellement une stratégie que le robot utilisera.",
    manual_cockpit_strategy_act: "Activation de la stratégie :",
    manual_cockpit_strategy_act_desc: "Le petit commutateur \"lecture/pause\" à côté de chaque stratégie vous permet d'activer ou de suspendre la stratégie pour ce symbole spécifique. Cela vous donne un contrôle précis si vous souhaitez rester en dehors d'un marché temporairement.",
    manual_cockpit_global: "Bouton du système global :",
    manual_cockpit_global_desc: "Le grand commutateur en haut à droite agit comme un interrupteur principal. S'il est désactivé, aucune transaction ne passera. Activez-le lorsque vous êtes prêt à laisser le robot fonctionner librement en fonction des stratégies assignées.",

    manual_monitoring_title: "Surveillance",
    manual_monitoring_p1: "La page de Surveillance offre une vue d'ensemble de la santé de votre compte et des actions en temps réel du robot.",
    manual_monitoring_top: "Tableau de bord :",
    manual_monitoring_top_desc: "Il présente un résumé du solde de votre compte, des pertes et profits du jour, et du nombre de transactions exécutées par le robot aujourd'hui.",
    manual_monitoring_status: "État du système :",
    manual_monitoring_status_desc: "Affiche si le moteur interne détecte des anomalies de données, et si le calcul global des règles s'exécute correctement.",
    manual_monitoring_logs: "Journaux d'activité :",
    manual_monitoring_logs_desc: "Au lieu de fouiller dans des fichiers texte techniques, les décisions importantes du robot (comme les soumissions d'ordres, les transactions remplies ou les erreurs) apparaîtront dans une liste claire ici.",
    manual_monitoring_when: "Quand l'utiliser :",
    manual_monitoring_when_desc: "Gardez cette page ouverte sur un deuxième écran pendant que le robot fonctionne pour vous assurer passivement que tout se passe bien.",

    manual_watchlist_title: "Liste de surveillance",
    manual_watchlist_p1: "La Liste de surveillance (Watchlist) agit comme un carnet d'adresses des symboles de marché que le robot suit activement. Le robot a besoin de charger les données pour les symboles avant de pouvoir les trader.",
    manual_watchlist_manual: "Entrées manuelles :",
    manual_watchlist_manual_desc: "Vous pouvez ajouter des symboles un par un en toute sécurité dans des groupes personnalisés pour les organiser (par exemple, Actions technologiques, Matières premières).",
    manual_watchlist_feeds: "Flux et Imports :",
    manual_watchlist_feeds_desc: "Vous pouvez coller une URL externe (comme un lien de screener TradingView) ou télécharger un fichier CSV pour remplir automatiquement votre liste de surveillance avec des centaines de tickers instantanément.",
    manual_watchlist_visibility: "Visibilité :",
    manual_watchlist_visibility_desc: "Les symboles ajoutés ici deviendront disponibles pour sélection dans votre Cockpit.",

    manual_strategy_title: "Générateur de stratégie",
    manual_strategy_p1: "Le Générateur de stratégie (Strategy Builder) vous permet de créer l'intelligence et les règles réelles que le robot utilise pour prendre des décisions.",
    manual_strategy_indicators: "Indicateurs :",
    manual_strategy_indicators_desc: "Définissez les outils mathématiques (comme les moyennes mobiles ou le RSI) que vous souhaitez que la stratégie prenne en compte.",
    manual_strategy_entry: "Règles d'entrée :",
    manual_strategy_entry_desc: "Définissez les conditions exactes qui doivent être remplies pour que le robot achète un actif (par exemple, 'Acheter lorsque le prix franchit la moyenne mobile').",
    manual_strategy_exit: "Règles de sortie :",
    manual_strategy_exit_desc: "Définissez quand le robot doit vendre un actif pour prendre ses bénéfices ou limiter ses pertes.",
    manual_strategy_validation: "Validation :",
    manual_strategy_validation_desc: "Le bouton en haut à droite vous permet de tester la logique de votre stratégie pour vous assurer qu'il n'y a pas de fautes de frappe ou de conditions impossibles avant de la déployer avec de l'argent réel.",
`;

let parts = content.split('export const translations = {');
let frIndex = parts[1].indexOf('fr: {');
let enPart = parts[1].substring(0, frIndex);
let frPart = parts[1].substring(frIndex);

enPart = enPart.replace('user_manual: "User Manual",', 'user_manual: "User Manual",\n' + enKeys);
frPart = frPart.replace('user_manual: "Manuel de l\'utilisateur",', 'user_manual: "Manuel de l\'utilisateur",\n' + frKeys);

fs.writeFileSync('src/lib/i18n/translations.ts', parts[0] + 'export const translations = {' + enPart + frPart);

