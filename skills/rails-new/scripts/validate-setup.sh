#!/bin/bash
# Post-setup validation for Rails application
# Checks that all components are properly installed and configured.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
WARN=0

pass() {
  echo -e "${GREEN}[PASS]${NC} $1"
  ((PASS++))
}

fail() {
  echo -e "${RED}[FAIL]${NC} $1"
  ((FAIL++))
}

warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
  ((WARN++))
}

echo "=== Rails Application Setup Validation ==="
echo ""

# 1. Check Gemfile has required gems
echo "--- Checking Gems ---"
for gem in devise interactor-rails sidekiq slim-rails pundit rspec-rails factory_bot_rails shoulda-matchers faker redis rubocop-rails rubocop-rspec dotenv-rails database_cleaner-active_record simplecov letter_opener; do
  if grep -q "gem ['\"]${gem}['\"]" Gemfile 2>/dev/null; then
    pass "Gem: $gem"
  else
    fail "Gem: $gem not found in Gemfile"
  fi
done

echo ""

# 2. Check bundle is installed
echo "--- Checking Bundle ---"
if bundle check > /dev/null 2>&1; then
  pass "Bundle is installed"
else
  fail "Bundle not installed (run: bundle install)"
fi

echo ""

# 3. Check JavaScript dependencies
echo "--- Checking JavaScript ---"
if [ -f "package.json" ]; then
  pass "package.json exists"
else
  fail "package.json missing"
fi

for pkg in "@hotwired/turbo-rails" "@hotwired/stimulus" "esbuild"; do
  if grep -q "\"${pkg}\"" package.json 2>/dev/null; then
    pass "npm: $pkg"
  else
    fail "npm: $pkg not found in package.json"
  fi
done

if [ -d "node_modules" ]; then
  pass "node_modules exists"
else
  fail "node_modules missing (run: yarn install)"
fi

echo ""

# 4. Check file structure
echo "--- Checking File Structure ---"

# Slim layout
if [ -f "app/views/layouts/application.html.slim" ]; then
  pass "Slim layout exists"
else
  fail "Slim layout missing (app/views/layouts/application.html.slim)"
fi

if [ -f "app/views/layouts/application.html.erb" ]; then
  warn "Old ERB layout still exists — should be removed"
fi

# RSpec
if [ -f ".rspec" ]; then
  pass ".rspec config exists"
else
  fail ".rspec missing (run: rails generate rspec:install)"
fi

if [ -d "spec" ]; then
  pass "spec/ directory exists"
else
  fail "spec/ directory missing"
fi

for dir in spec/factories spec/models spec/requests spec/interactors spec/support; do
  if [ -d "$dir" ]; then
    pass "Directory: $dir"
  else
    fail "Directory: $dir missing"
  fi
done

# Stimulus controllers
if [ -f "app/javascript/controllers/application.js" ]; then
  pass "Stimulus application.js exists"
else
  fail "Stimulus application.js missing"
fi

if [ -f "app/javascript/controllers/index.js" ]; then
  pass "Stimulus index.js exists"
else
  fail "Stimulus index.js missing"
fi

# Interactors
if [ -d "app/interactors" ]; then
  pass "Interactors directory exists"
else
  fail "Interactors directory missing"
fi

# Sidekiq config
if [ -f "config/sidekiq.yml" ]; then
  pass "Sidekiq config exists"
else
  fail "Sidekiq config missing"
fi

# Pundit
if [ -f "app/policies/application_policy.rb" ]; then
  pass "Pundit application policy exists"
else
  fail "Pundit application policy missing (run: rails generate pundit:install)"
fi

# Procfile
if [ -f "Procfile.dev" ]; then
  pass "Procfile.dev exists"
  if grep -q "sidekiq" Procfile.dev; then
    pass "Procfile.dev includes Sidekiq worker"
  else
    fail "Procfile.dev missing Sidekiq worker entry"
  fi
else
  fail "Procfile.dev missing"
fi

# dotenv
if [ -f ".env.example" ]; then
  pass ".env.example exists"
else
  warn ".env.example missing"
fi

# RuboCop
if [ -f ".rubocop.yml" ]; then
  pass ".rubocop.yml exists"
else
  warn ".rubocop.yml missing"
fi

# Devise
if [ -f "config/initializers/devise.rb" ]; then
  pass "Devise initializer exists"
else
  fail "Devise not installed (run: rails generate devise:install)"
fi

if [ -f "app/models/user.rb" ]; then
  if grep -q "devise" app/models/user.rb; then
    pass "User model has Devise"
  else
    fail "User model missing Devise modules"
  fi
else
  fail "User model missing (run: rails generate devise User)"
fi

echo ""

# 5. Check generators config
echo "--- Checking Configuration ---"
if grep -q "template_engine.*:slim" config/application.rb 2>/dev/null; then
  pass "Generator configured for Slim"
else
  fail "Generator not configured for Slim in config/application.rb"
fi

if grep -q "queue_adapter.*:sidekiq" config/application.rb 2>/dev/null; then
  pass "Active Job configured for Sidekiq"
else
  fail "Active Job not configured for Sidekiq in config/application.rb"
fi

echo ""

# 6. Run RSpec
echo "--- Running Tests ---"
if bundle exec rspec --no-color 2>&1; then
  pass "RSpec suite passes"
else
  fail "RSpec suite has failures"
fi

echo ""

# 7. Check JS builds
echo "--- Checking JS Build ---"
if yarn build 2>&1; then
  pass "yarn build succeeds"
else
  fail "yarn build fails"
fi

echo ""

# Summary
echo "=== Validation Summary ==="
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${RED}Failed: $FAIL${NC}"
echo -e "${YELLOW}Warnings: $WARN${NC}"
echo ""

if [ $FAIL -gt 0 ]; then
  echo -e "${RED}Setup has issues that need to be resolved.${NC}"
  exit 1
else
  echo -e "${GREEN}Setup is complete!${NC}"
  exit 0
fi
