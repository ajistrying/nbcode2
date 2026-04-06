# Interactor Service Objects Setup

## Overview

The `interactor-rails` gem provides a service object pattern for encapsulating
business logic. Each interactor does one thing, receives a context, and either
succeeds or fails.

## Directory

```
app/interactors/
```

Rails autoloads this directory automatically with `interactor-rails`.

## Base Concern

Create `app/interactors/application_interactor.rb` for shared behavior:

```ruby
module ApplicationInteractor
  extend ActiveSupport::Concern

  included do
    # Add shared behavior across all interactors:
    # - Logging
    # - Error wrapping
    # - Instrumentation
  end
end
```

## Single Interactor Pattern

```ruby
# app/interactors/authenticate_user.rb
class AuthenticateUser
  include Interactor
  include ApplicationInteractor

  delegate :email, :password, to: :context

  def call
    user = User.find_by(email: email)

    if user&.valid_password?(password)
      context.user = user
      context.token = generate_token(user)
    else
      context.fail!(error: "Invalid credentials")
    end
  end

  private

  def generate_token(user)
    # Token generation logic
  end
end
```

## Organizer Pattern (Multi-Step Workflows)

```ruby
# app/interactors/register_user.rb
class RegisterUser
  include Interactor::Organizer
  include ApplicationInteractor

  organize CreateUser, SendWelcomeEmail, TrackSignup
end
```

Each step runs in order. If any step fails, subsequent steps are skipped and
previous steps' `rollback` methods are called (if defined).

## Testing Interactors

```ruby
# spec/interactors/create_user_spec.rb
require "rails_helper"

RSpec.describe CreateUser, type: :interactor do
  describe ".call" do
    context "with valid params" do
      let(:result) { described_class.call(email: "test@example.com", password: "password123!") }

      it "succeeds" do
        expect(result).to be_a_success
      end

      it "provides the created user" do
        expect(result.user).to be_persisted
        expect(result.user.email).to eq("test@example.com")
      end
    end

    context "with invalid params" do
      let(:result) { described_class.call(email: "", password: "password123!") }

      it "fails" do
        expect(result).to be_a_failure
      end

      it "provides error messages" do
        expect(result.errors).to include(/email/i)
      end
    end
  end
end
```

## Conventions

- One public action per interactor (the `call` method)
- Use `context.fail!` to indicate failure (never raise for business logic errors)
- Use `delegate` to pull named values from context for readability
- Use Organizers to compose multi-step workflows
- Keep interactors focused — if `call` is longer than ~20 lines, decompose
- Always test both success and failure paths
