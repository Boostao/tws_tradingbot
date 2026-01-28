# Trading bot using Trader Workstation Python API by Interactive Brokers

 This goal of this project is to create a trading bot.

 streamlit UI to monitor logging, API calls, interact with the bot.

 Retrieve list of stocks to work with from TWS wishlist, selectable from streamlit UI.

 Configurable starting assets on bot boot.

 Include unit testing.

 Separate buy trading decision logic ( a single function that gets fed with stock ticker and return a buy or do nothing action). Only runs on stock that do not have an opened position from the bot.

 Separate sell trading decision logic ( a single function that gets fed with stock ticker and return a sell or do nothing action). Only runs on stock that have a opened position from the bot.

 Equal assets allocation on buy action, i.e. if 20 stocks in the startup list, and 100k starting assets, buy action should have no more than 5k volume order.

 Bot works in a loop, on each stock, every minutes.

 Always refresh the current stock list from the wishlist.

 Display the currently followed stock in the streamlit ui with their status : No position, Opened Position with volume in parenthesis.

 Always on display turn on, turn off bot from streamlit UI.

 Error recovery handling, bot is stateless, always picks info from TWS workstation, the source of truth.

 In the buy logic, no action if VIX price is over the EMA line in the past five minutes.

 Only trade during regular market hours, only use market hours data, no after market / market extension data.

 Traderworkstation is installed on the machine.

 Install the python TWS API, to tests thing out and all other dependencies for streamlit and the bot.
