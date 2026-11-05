# Deploy to Dokku

## Initial setup

On an Ubuntu server, install Dokku according to <https://dokku.com/docs/getting-started/installation/>, then:

```sh
dokku plugin:install https://github.com/dokku/dokku-mongo.git
```

```sh
APP=webhook-telegram-bot
dokku apps:create $APP

dokku mongo:create $APP
dokku mongo:link $APP $APP

dokku config:set $APP GITHUB_TELEGRAM_BOT_TOKEN=
dokku config:set $APP GITLAB_TELEGRAM_BOT_TOKEN=
dokku config:set $APP SECRET_KEY=$(openssl rand -hex 32)
dokku config:set $APP BASE_URL=https://webhook.git-watch.mobicom.dev

dokku domains:set $APP webhook.git-watch.mobicom.dev
```

Locally:

```sh
git push dokku@dokku.host:$APP
```

If Dokku runs nginx, configure LetsEncrypt (not needed when using Traefik):

```sh
dokku plugin:install https://github.com/dokku/dokku-letsencrypt.git
dokku letsencrypt:cron-job --add
dokku letsencrypt:set $APP email your@email.com
dokku letsencrypt:enable $APP
```
