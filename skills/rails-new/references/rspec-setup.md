# RSpec Testing Setup

## Installation

```bash
rails generate rspec:install
```

Creates:
- `.rspec` — CLI options
- `spec/spec_helper.rb` — Pure Ruby spec config
- `spec/rails_helper.rb` — Rails-specific spec config

## .rspec Configuration

```
--require spec_helper
--format documentation
--color
```

## spec/spec_helper.rb

Add SimpleCov at the very top (before ALL other requires):

```ruby
require "simplecov"
SimpleCov.start "rails" do
  add_filter "/spec/"
  add_filter "/config/"
  add_filter "/vendor/"
end
```

## spec/rails_helper.rb Additions

### Load support files (after the Rails require)

```ruby
Dir[Rails.root.join("spec/support/**/*.rb")].each { |f| require f }
```

### Inside RSpec.configure block

```ruby
# FactoryBot shorthand methods (create, build, etc.)
config.include FactoryBot::Syntax::Methods

# Devise test helpers for request specs
config.include Devise::Test::IntegrationHelpers, type: :request
config.include Devise::Test::IntegrationHelpers, type: :system

# DatabaseCleaner
config.before(:suite) do
  DatabaseCleaner.strategy = :transaction
  DatabaseCleaner.clean_with(:truncation)
end

config.around(:each) do |example|
  DatabaseCleaner.cleaning do
    example.run
  end
end
```

### Shoulda Matchers (bottom of file, outside RSpec.configure)

```ruby
Shoulda::Matchers.configure do |config|
  config.integrate do |with|
    with.test_framework :rspec
    with.library :rails
  end
end
```

## Directory Structure

```
spec/
  factories/        # FactoryBot factories
  interactors/      # Interactor specs
  models/           # Model specs
  requests/         # Request/controller specs
  support/          # Shared config and helpers
  system/           # System/integration specs (browser)
  spec_helper.rb
  rails_helper.rb
```

## Generator Config

Rails generators will automatically create RSpec specs instead of Minitest tests
because `rspec-rails` is installed. No additional generator config needed.

## Running Tests

```bash
# Full suite
bundle exec rspec

# Specific directory
bundle exec rspec spec/models

# Specific file
bundle exec rspec spec/models/user_spec.rb

# Specific test by line number
bundle exec rspec spec/models/user_spec.rb:5
```
