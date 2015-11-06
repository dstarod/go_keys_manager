###Twitter REST API keys provider

Visit [apps.twitter.com](https://apps.twitter.com) for details.

Make JSON file with accounts, for example /path/to/accounts.json:

	[
		{
			"consumer_key": "your-consumer-key",
			"consumer_secret" : "your-consumer-secret",
			"access_token": "your-access-token",
			"access_token_secret": "your-access-secret"
		},
		{
			... another key ...
		}
	]

Start application with flags "port" and "keys_file", for example:

	./go_keys_manager --port 7777 --keys_file /path/to/accounts.json

Get key example:

	GET http://localhost:7777/get?service=search/tweets

Set key example:

	POST http://localhost:7777/set?service=search/tweets
	
Form fields:

- consumer_key string (requred)
- consumer_secret string (requred)
- access_token string (requred)
- access_token_secret string (requred)
- remaining int (current rate limit, optional but useful for next usage)
- reset int (next rate limit reset UNIX time, optional but useful for next usage)