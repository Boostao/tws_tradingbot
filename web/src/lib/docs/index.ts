import type { DocMeta, DocRecord } from './types';

const docs: DocRecord[] = [
	{
		slug: 'quickstart',
		titleKey: 'doc_quickstart_title',
		descriptionKey: 'doc_quickstart_desc',
		order: 1,
		sections: [
			{ type: 'paragraph', key: 'doc_quickstart_intro' },
			{ type: 'heading', key: 'doc_quickstart_steps_title' },
			{
				type: 'list',
				keys: [
					'doc_quickstart_step_1',
					'doc_quickstart_step_2',
					'doc_quickstart_step_3',
					'doc_quickstart_step_4',
					'doc_quickstart_step_5',
					'doc_quickstart_step_6'
				]
			}
		]
	},
	{
		slug: 'strategy-guide',
		titleKey: 'doc_strategy_guide_title',
		descriptionKey: 'doc_strategy_guide_desc',
		order: 2,
		sections: [
			{ type: 'paragraph', key: 'doc_strategy_intro' },
			{ type: 'heading', key: 'doc_strategy_flow_title' },
			{ type: 'paragraph', key: 'doc_strategy_flow_text' },
			{ type: 'heading', key: 'doc_strategy_scopes_title' },
			{ type: 'list', keys: ['doc_strategy_scope_global', 'doc_strategy_scope_ticker'] },
			{ type: 'heading', key: 'doc_strategy_actions_title' },
			{ type: 'list', keys: ['doc_strategy_action_buy', 'doc_strategy_action_sell', 'doc_strategy_action_filter'] },
			{ type: 'heading', key: 'doc_strategy_indicators_title' },
			{ type: 'list', keys: ['doc_strategy_indicators_list'] },
			{ type: 'heading', key: 'doc_strategy_conditions_title' },
			{ type: 'list', keys: ['doc_strategy_conditions_list'] },
			{ type: 'heading', key: 'doc_strategy_examples_title' },
			{ type: 'list', keys: ['doc_strategy_example_ema', 'doc_strategy_example_vix'] },
			{ type: 'heading', key: 'doc_strategy_best_practices_title' },
			{ type: 'list', keys: ['doc_strategy_best_1', 'doc_strategy_best_2', 'doc_strategy_best_3', 'doc_strategy_best_4'] }
		]
	},
	{
		slug: 'tws-notes',
		titleKey: 'doc_tws_notes_title',
		descriptionKey: 'doc_tws_notes_desc',
		order: 3,
		sections: [
			{ type: 'paragraph', key: 'doc_tws_intro' },
			{ type: 'heading', key: 'doc_tws_connection_title' },
			{ type: 'list', keys: ['doc_tws_connection_1', 'doc_tws_connection_2', 'doc_tws_connection_3'] },
			{ type: 'heading', key: 'doc_tws_data_title' },
			{ type: 'list', keys: ['doc_tws_data_1', 'doc_tws_data_2'] },
			{ type: 'heading', key: 'doc_tws_sessions_title' },
			{ type: 'list', keys: ['doc_tws_sessions_1', 'doc_tws_sessions_2'] },
			{ type: 'heading', key: 'doc_tws_pacing_title' },
			{ type: 'list', keys: ['doc_tws_pacing_1', 'doc_tws_pacing_2'] },
			{ type: 'heading', key: 'doc_tws_errors_title' },
			{ type: 'list', keys: ['doc_tws_errors_1', 'doc_tws_errors_2'] },
			{ type: 'heading', key: 'doc_tws_env_title' },
			{ type: 'list', keys: ['doc_tws_env_1', 'doc_tws_env_2'] }
		]
	},
	{
		slug: 'configuration',
		titleKey: 'doc_configuration_title',
		descriptionKey: 'doc_configuration_desc',
		order: 4,
		sections: [
			{ type: 'heading', key: 'doc_configuration_files_title' },
			{ type: 'list', keys: ['doc_configuration_file_1', 'doc_configuration_file_2', 'doc_configuration_file_3'] },
			{ type: 'heading', key: 'doc_configuration_env_title' },
			{
				type: 'code',
				language: 'text',
				code:
					'IB_HOST=127.0.0.1\nIB_PORT=7497\nIB_CLIENT_ID=1\nIB_ACCOUNT=your_account_id'
			},
			{ type: 'heading', key: 'doc_configuration_scripts_title' },
			{ type: 'list', keys: ['doc_configuration_script_1', 'doc_configuration_script_2', 'doc_configuration_script_3'] },
			{ type: 'heading', key: 'doc_configuration_data_title' },
			{ type: 'list', keys: ['doc_configuration_data_1', 'doc_configuration_data_2'] },
			{ type: 'heading', key: 'doc_configuration_backtest_title' },
			{ type: 'list', keys: ['doc_configuration_backtest_1', 'doc_configuration_backtest_2'] }
		]
	},
	{
		slug: 'architecture',
		titleKey: 'doc_architecture_title',
		descriptionKey: 'doc_architecture_desc',
		order: 5,
		sections: [
			{ type: 'paragraph', key: 'doc_architecture_intro' },
			{
				type: 'mermaid',
				code:
					'graph TD\n    A[User] --> B[SvelteKit UI]\n    B --> C[FastAPI API]\n    C --> D[Bot Engine]\n    C --> E[WebSocket Streams]\n    E --> B\n\n    D --> F[Live Runner]\n    D --> G[Backtest Runner]\n    D --> H[Optimizer]\n\n    F --> I[Strategy System]\n    G --> I\n    H --> I\n\n    I --> J[Rules Engine]\n    J --> K[Conditions]\n    J --> L[Indicators]\n    J --> M[Evaluator]\n\n    F --> N[Order Manager]\n    F --> O[Risk Manager]\n    F --> P[State Manager]\n\n    N --> Q[IB Adapter]\n    Q --> R[TWS API]\n\n    P --> S[Database]\n    S --> T[DuckDB]\n\n    D --> V[Data Providers]\n    V --> W[TWS Data Provider]\n    V --> X[Historical Data Loader]\n\n    C --> Y[API Routers]\n    Y --> Z[Strategy]\n    Y --> AA[Backtest]\n    Y --> BB[Config]\n    Y --> CC[State]\n    Y --> DD[Symbols]\n    Y --> EE[Watchlist]\n    Y --> FF[Notifications]\n\n    B --> GG[Components]\n    GG --> HH[Strategy Builder]\n    GG --> II[Monitoring Dashboard]\n    GG --> JJ[Backtest UI]\n    GG --> KK[Watchlist Manager]\n\n    D --> LL[Notifications Service]\n    LL --> MM[Telegram]\n    LL --> NN[Discord]\n\n    subgraph "External Services"\n        R\n        MM\n        NN\n    end\n\n    subgraph "Data Storage"\n        T\n    end\n\n    subgraph "Configuration"\n        OO[Config Files]\n        OO --> PP[default.yaml]\n        OO --> QQ[active_strategy.json]\n    end'
			}
		]
	}
];

export const getDocList = () => docs;

export const getDocBySlug = (slug: string) => docs.find((doc) => doc.slug === slug) ?? null;
