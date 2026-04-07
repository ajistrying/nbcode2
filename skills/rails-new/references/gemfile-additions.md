# Gemfile Additions

These gems should be added to the Gemfile after initial Rails generation.

## Main Gems (no group)

```ruby
# Authentication
gem "devise"

# Authorization
gem "pundit"

# Service objects
gem "interactor-rails"

# Background jobs
gem "sidekiq"

# Redis (for Sidekiq and caching)
gem "redis"

# Slim templating
gem "slim-rails"
```

## Development + Test Group

```ruby
group :development, :test do
  # Testing framework
  gem "rspec-rails"

  # Test factories
  gem "factory_bot_rails"

  # Fake data generation
  gem "faker"

  # Environment variables
  gem "dotenv-rails"
end
```

## Test Group

```ruby
group :test do
  # One-liner matchers for common Rails patterns
  gem "shoulda-matchers"

  # Database cleaning between tests
  gem "database_cleaner-active_record"

  # Code coverage reports
  gem "simplecov", require: false
end
```

## Development Group (add to existing)

```ruby
group :development do
  # Linting
  gem "rubocop-rails", require: false
  gem "rubocop-rspec", require: false

  # Preview emails in browser instead of sending
  gem "letter_opener"
end
```

## Notes

- Do NOT pin to specific versions unless there's a known compatibility issue. Let Bundler resolve the latest compatible versions.
- The `pg` gem is already included by `--database=postgresql`.
- `tailwindcss-rails` is already included by `--css=tailwind`.
- `jsbundling-rails` is already included by `--javascript=esbuild`.
- `turbo-rails` and `stimulus-rails` are included by default in Rails 7+.
