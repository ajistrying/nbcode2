# Sidekiq Background Jobs Setup

## Active Job Adapter

In `config/application.rb`:

```ruby
config.active_job.queue_adapter = :sidekiq
```

## Sidekiq Configuration File

Create `config/sidekiq.yml`:

```yaml
:concurrency: 5
:queues:
  - default
  - mailers
  - [low_priority, 1]
```

## Redis Connection

Sidekiq connects to Redis at `localhost:6379` by default. For custom config, create
`config/initializers/sidekiq.rb`:

```ruby
Sidekiq.configure_server do |config|
  config.redis = { url: ENV.fetch("REDIS_URL", "redis://localhost:6379/0") }
end

Sidekiq.configure_client do |config|
  config.redis = { url: ENV.fetch("REDIS_URL", "redis://localhost:6379/0") }
end
```

## Web UI

Add to `config/routes.rb` (protected by Devise authentication):

```ruby
require "sidekiq/web"

Rails.application.routes.draw do
  authenticate :user do
    mount Sidekiq::Web => "/sidekiq"
  end
  # ...
end
```

For admin-only access, add an admin check:

```ruby
authenticate :user, ->(user) { user.admin? } do
  mount Sidekiq::Web => "/sidekiq"
end
```

(This requires an `admin` boolean column on the User model — skip for initial setup.)

## Procfile.dev

Sidekiq needs its own process. Add to `Procfile.dev`:

```
worker: bundle exec sidekiq -C config/sidekiq.yml
```

The full Procfile.dev should look like:

```
web: bin/rails server -p 3000
js: yarn build --watch
css: bin/rails tailwindcss:watch
worker: bundle exec sidekiq -C config/sidekiq.yml
```

## Running Locally

```bash
# Start all processes (requires foreman or similar)
bin/dev

# Or start Sidekiq standalone
bundle exec sidekiq -C config/sidekiq.yml
```

## Troubleshooting

- **Redis not running**: Sidekiq requires Redis. If it fails to connect, the user
  needs to start Redis (`brew services start redis` on macOS). Do NOT start services
  automatically — inform the user.
- **Jobs not processing**: Check that the queue names in the job class match those in
  `sidekiq.yml`.
