---
name: rails-new
description: >
  Scaffold a modern Rails application with PostgreSQL, Devise authentication,
  Sidekiq background jobs, Interactor service objects, RSpec testing, Tailwind CSS,
  Hotwire (Turbo + Stimulus) with ESBuild, and Slim templating. Invoke when user
  says "new rails app", "scaffold rails", "start a rails project", or similar.
disable-model-invocation: true
allowed-tools: Bash Read Write Edit Glob Grep
argument-hint: <app-name>
context: fork
---

# Rails Application Setup

Create a new Rails application named `$0` with the full modern stack.

> **Important**: Execute each step sequentially. If any step fails, diagnose and fix
> before moving on. Do NOT skip steps. Verify each gem installation and configuration
> actually works before proceeding.

---

## Step 1: Generate the Rails Application

```bash
rails new $0 \
  --database=postgresql \
  --css=tailwind \
  --javascript=esbuild \
  --skip-test \
  --skip-jbuilder
```

This gives us:
- PostgreSQL via `pg` gem
- Tailwind CSS via `tailwindcss-rails`
- ESBuild via `jsbundling-rails` (with `@hotwired/turbo-rails` and `@hotwired/stimulus` as npm packages)
- Skips Minitest (we use RSpec)
- Skips Jbuilder (not needed for a Hotwire app)

After generation, `cd` into the app directory for all remaining steps.

---

## Step 2: Verify ESBuild + Hotwire Setup

Before adding anything, confirm the JavaScript pipeline is correct.

**Check `package.json`** has these dependencies:
- `@hotwired/turbo-rails`
- `@hotwired/stimulus`
- `esbuild`

If any are missing:
```bash
yarn add @hotwired/turbo-rails @hotwired/stimulus
```

**Check `app/javascript/application.js`** imports Turbo and Stimulus:
```javascript
import "@hotwired/turbo-rails"
import "./controllers"
```

**Check `app/javascript/controllers/index.js`** has the Stimulus application setup:
```javascript
import { application } from "./application"
// Eager-load all controllers in this directory
import { eagerLoadControllersFrom } from "@hotwired/stimulus-loading"
eagerLoadControllersFrom("controllers", application)
```

**Check `app/javascript/controllers/application.js`** creates the Stimulus app:
```javascript
import { Application } from "@hotwired/stimulus"
const application = Application.start()
application.debug = false
window.Stimulus = application
export { application }
```

If the Stimulus controller structure doesn't exist or is incomplete, create it manually using the files above.

**Create a sample Stimulus controller** to validate the pipeline works:

Write `app/javascript/controllers/hello_controller.js`:
```javascript
import { Controller } from "@hotwired/stimulus"

export default class extends Controller {
  connect() {
    console.log("Hello controller connected")
  }
}
```

---

## Step 3: Add Gems to Gemfile

Open the `Gemfile` and add the following gems. See `references/gemfile-additions.md` for the full list with version constraints and grouping.

**Main gems** (outside any group):
```ruby
gem "devise"
gem "interactor-rails"
gem "sidekiq"
gem "slim-rails"
gem "pundit"
gem "redis"
```

**Development/Test group**:
```ruby
group :development, :test do
  gem "rspec-rails"
  gem "factory_bot_rails"
  gem "faker"
  gem "dotenv-rails"
end
```

**Test group**:
```ruby
group :test do
  gem "shoulda-matchers"
  gem "database_cleaner-active_record"
  gem "simplecov", require: false
end
```

**Development group** (add to existing):
```ruby
group :development do
  gem "rubocop-rails", require: false
  gem "rubocop-rspec", require: false
  gem "letter_opener"
end
```

Then run:
```bash
bundle install
```

If there are any version conflicts, resolve them by relaxing version constraints or checking gem compatibility. Do not proceed until `bundle install` succeeds cleanly.

---

## Step 4: Configure Slim Templating

See `references/slim-setup.md` for full details.

**4a.** Add Slim generator config to `config/application.rb` inside the `Application` class:

```ruby
config.generators do |g|
  g.template_engine :slim
end
```

**4b.** Convert `app/views/layouts/application.html.erb` to `app/views/layouts/application.html.slim`.

The Slim layout should be:

```slim
doctype html
html
  head
    title= content_for(:title) || "$0"
    meta name="viewport" content="width=device-width,initial-scale=1"
    meta name="apple-mobile-web-app-capable" content="yes"
    = csrf_meta_tags
    = csp_meta_tag
    = stylesheet_link_tag "tailwind", "inter-font", "data-turbo-track": "reload"
    = stylesheet_link_tag "application", "data-turbo-track": "reload"
    = javascript_include_tag "application", "data-turbo-track": "reload", type: "module"
  body
    - if notice.present?
      p.notice = notice
    - if alert.present?
      p.alert = alert
    = yield
```

**4c.** Delete the old `application.html.erb` file after creating the Slim version.

**4d.** If a `mailer.html.erb` layout exists, convert it to `mailer.html.slim` as well.

---

## Step 5: Configure RSpec

See `references/rspec-setup.md` for full details.

**5a.** Run the RSpec installer:
```bash
rails generate rspec:install
```

This creates `.rspec`, `spec/spec_helper.rb`, and `spec/rails_helper.rb`.

**5b.** Update `.rspec`:
```
--require spec_helper
--format documentation
--color
```

**5c.** Configure `spec/rails_helper.rb` — add the following inside the `RSpec.configure` block:

```ruby
# FactoryBot
config.include FactoryBot::Syntax::Methods

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

**5d.** Add Shoulda Matchers config at the bottom of `spec/rails_helper.rb`:

```ruby
Shoulda::Matchers.configure do |config|
  config.integrate do |with|
    with.test_framework :rspec
    with.library :rails
  end
end
```

**5e.** Add SimpleCov at the very top of `spec/spec_helper.rb` (before anything else):

```ruby
require "simplecov"
SimpleCov.start "rails" do
  add_filter "/spec/"
  add_filter "/config/"
  add_filter "/vendor/"
end
```

**5f.** Create the support directory structure:
```bash
mkdir -p spec/support spec/factories spec/models spec/requests spec/interactors spec/system
```

**5g.** Create `spec/support/factory_bot.rb`:
```ruby
RSpec.configure do |config|
  config.include FactoryBot::Syntax::Methods
end
```

**5h.** Add to `spec/rails_helper.rb` (near the top, after the Rails require):
```ruby
Dir[Rails.root.join("spec/support/**/*.rb")].each { |f| require f }
```

**5i.** Verify RSpec works:
```bash
bundle exec rspec
```

Should output `0 examples, 0 failures`.

---

## Step 6: Configure Devise Authentication

See `references/devise-setup.md` for full details.

**6a.** Run the Devise installer:
```bash
rails generate devise:install
```

**6b.** Follow the installer output. At minimum, configure the mailer default URL in `config/environments/development.rb`:
```ruby
config.action_mailer.default_url_options = { host: "localhost", port: 3000 }
```

**6c.** Generate the User model:
```bash
rails generate devise User
```

**6d.** Run the migration:
```bash
rails db:create
rails db:migrate
```

If `db:create` fails because PostgreSQL isn't running or the role doesn't exist, inform the user and provide the fix commands but do not attempt to start system services.

**6e.** Generate Devise views (so they can be customized, and they'll use Slim since we configured the generator):
```bash
rails generate devise:views
```

**6f.** If the generated views are `.erb` instead of `.slim`, convert them to Slim. The key views to convert are:
- `app/views/devise/sessions/new`
- `app/views/devise/registrations/new`
- `app/views/devise/registrations/edit`
- `app/views/devise/passwords/new`
- `app/views/devise/passwords/edit`
- `app/views/devise/shared/_links`
- `app/views/devise/shared/_error_messages`

**6g.** Create a basic RSpec test for the User model at `spec/models/user_spec.rb`:
```ruby
require "rails_helper"

RSpec.describe User, type: :model do
  describe "validations" do
    it { is_expected.to validate_presence_of(:email) }
    it { is_expected.to validate_uniqueness_of(:email).case_insensitive }
  end

  describe "devise modules" do
    it "includes database_authenticatable" do
      expect(User.devise_modules).to include(:database_authenticatable)
    end

    it "includes registerable" do
      expect(User.devise_modules).to include(:registerable)
    end

    it "includes recoverable" do
      expect(User.devise_modules).to include(:recoverable)
    end

    it "includes rememberable" do
      expect(User.devise_modules).to include(:rememberable)
    end

    it "includes validatable" do
      expect(User.devise_modules).to include(:validatable)
    end
  end
end
```

**6h.** Create a factory at `spec/factories/users.rb`:
```ruby
FactoryBot.define do
  factory :user do
    email { Faker::Internet.email }
    password { "password123!" }
    password_confirmation { "password123!" }
  end
end
```

---

## Step 7: Configure Sidekiq

See `references/sidekiq-config.md` for full details.

**7a.** Set Sidekiq as the Active Job adapter in `config/application.rb`:
```ruby
config.active_job.queue_adapter = :sidekiq
```

**7b.** Create `config/sidekiq.yml`:
```yaml
:concurrency: 5
:queues:
  - default
  - mailers
  - [low_priority, 1]
```

**7c.** Add Sidekiq Web UI to `config/routes.rb` (require authentication):
```ruby
require "sidekiq/web"

Rails.application.routes.draw do
  authenticate :user do
    mount Sidekiq::Web => "/sidekiq"
  end

  devise_for :users
  # root "home#index"
end
```

**7d.** Create a `Procfile.dev` if one doesn't exist, or update the existing one to include Sidekiq:
```
web: bin/rails server -p 3000
js: yarn build --watch
css: bin/rails tailwindcss:watch
worker: bundle exec sidekiq -C config/sidekiq.yml
```

---

## Step 8: Configure Interactor

See `references/interactor-setup.md` for full details.

**8a.** Create the interactors directory:
```bash
mkdir -p app/interactors
```

**8b.** Create a base interactor concern at `app/interactors/application_interactor.rb`:
```ruby
module ApplicationInteractor
  extend ActiveSupport::Concern

  included do
    # Add shared behavior here, e.g.:
    # - Logging
    # - Error wrapping
    # - Instrumentation
  end
end
```

**8c.** Create a sample interactor to demonstrate the pattern at `app/interactors/create_user.rb`:
```ruby
class CreateUser
  include Interactor
  include ApplicationInteractor

  delegate :email, :password, to: :context

  def call
    user = User.new(email: email, password: password, password_confirmation: password)

    if user.save
      context.user = user
    else
      context.fail!(errors: user.errors.full_messages)
    end
  end
end
```

**8d.** Create a spec at `spec/interactors/create_user_spec.rb`:
```ruby
require "rails_helper"

RSpec.describe CreateUser, type: :interactor do
  describe ".call" do
    context "with valid params" do
      let(:result) { described_class.call(email: "test@example.com", password: "password123!") }

      it "succeeds" do
        expect(result).to be_a_success
      end

      it "creates a user" do
        expect(result.user).to be_persisted
      end
    end

    context "with invalid params" do
      let(:result) { described_class.call(email: "", password: "password123!") }

      it "fails" do
        expect(result).to be_a_failure
      end

      it "provides error messages" do
        expect(result.errors).to be_present
      end
    end
  end
end
```

---

## Step 9: Configure Pundit Authorization

**9a.** Run the Pundit generator:
```bash
rails generate pundit:install
```

This creates `app/policies/application_policy.rb`.

**9b.** Include Pundit in `ApplicationController`:
```ruby
class ApplicationController < ActionController::Base
  include Pundit::Authorization
end
```

---

## Step 10: Configure dotenv

**10a.** Create `.env` at the project root:
```
DATABASE_URL=postgresql://localhost/$0_development
REDIS_URL=redis://localhost:6379/0
```

**10b.** Add `.env` to `.gitignore` if not already there.

**10c.** Create `.env.example` with the same keys but empty/placeholder values:
```
DATABASE_URL=postgresql://localhost/myapp_development
REDIS_URL=redis://localhost:6379/0
```

---

## Step 11: Configure RuboCop

**11a.** Create `.rubocop.yml` at the project root:
```yaml
require:
  - rubocop-rails
  - rubocop-rspec

AllCops:
  NewCops: enable
  TargetRubyVersion: 3.2
  Exclude:
    - "bin/**/*"
    - "db/schema.rb"
    - "node_modules/**/*"
    - "vendor/**/*"
    - "config/environments/**/*"
    - "config/initializers/devise.rb"

Style/Documentation:
  Enabled: false

Style/FrozenStringLiteralComment:
  Enabled: false

Metrics/BlockLength:
  Exclude:
    - "spec/**/*"
    - "config/routes.rb"
    - "config/environments/**/*"

RSpec/ExampleLength:
  Max: 10

RSpec/MultipleExpectations:
  Max: 3
```

---

## Step 12: Create a Home Page

Every Rails app needs a root route. Create a minimal home page.

**12a.** Generate the controller:
```bash
rails generate controller Home index --skip-routes --no-helper --no-test-framework
```

**12b.** The generated view will be `.html.slim` (because of our generator config). If it's `.erb`, convert it. Write `app/views/home/index.html.slim`:
```slim
div.min-h-screen.flex.items-center.justify-center
  div.text-center
    h1.text-4xl.font-bold.mb-4 $0
    p.text-gray-600 Your application is running.
    - if user_signed_in?
      p.mt-4
        = link_to "Sign Out", destroy_user_session_path, data: { turbo_method: :delete }, class: "text-blue-600 hover:underline"
    - else
      p.mt-4
        = link_to "Sign In", new_user_session_path, class: "text-blue-600 hover:underline"
        |  or 
        = link_to "Sign Up", new_user_registration_path, class: "text-blue-600 hover:underline"
```

**12c.** Uncomment or add the root route in `config/routes.rb`:
```ruby
root "home#index"
```

---

## Step 13: Create the Database and Run Migrations

```bash
rails db:create
rails db:migrate
```

If this fails, do NOT try to start PostgreSQL or create roles. Instead, inform the user what needs to happen and let them handle it.

---

## Step 14: Run the Full Test Suite

```bash
bundle exec rspec
```

All tests should pass. If any fail, fix the issues before proceeding.

---

## Step 15: Initialize Git and Create Initial Commit

```bash
git init
git add -A
git commit -m "Initial Rails application setup

Stack:
- Rails with PostgreSQL
- Tailwind CSS
- ESBuild + Hotwire (Turbo + Stimulus)
- Slim templating
- Devise authentication
- Sidekiq background jobs
- Interactor service objects
- Pundit authorization
- RSpec test suite
- RuboCop linting"
```

---

## Step 16: Final Validation

Run the validation script to confirm everything is wired up:
```bash
bash ${CLAUDE_SKILL_DIR}/scripts/validate-setup.sh
```

If the validation script is not available, manually verify:
1. `bundle exec rspec` passes
2. `yarn build` completes without errors
3. `bundle exec rubocop --parallel` runs (warnings OK, no errors)
4. `bin/rails routes` shows Devise and Sidekiq routes
5. The Procfile.dev has all four processes (web, js, css, worker)

Report the results to the user with any warnings or manual steps needed (e.g., starting PostgreSQL, Redis).
