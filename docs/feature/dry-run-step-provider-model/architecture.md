mermaid
graph TD
    User -- dry-run command --> CLI(cmd/nexus-weaver/main.go)
    CLI -- Initializes & Calls --> EngineRunner(internal/engine/runner.go)
    EngineRunner -- Loads --> WorkflowDefinition(YAML Workflow)
    WorkflowDefinition -- Parses into --> Workflow(internal/engine/workflow.go::Workflow struct)
    Workflow -- Iterates Steps --> EngineRunner
    EngineRunner -- For each Step --> Step(internal/engine/workflow.go::Step struct)
    Step -- Contains LLM config (if any) --> EngineRunner
    EngineRunner -- Formats Output with LLM Info --> CLI
    CLI -- Displays Formatted Output --> User

    subgraph LLM Component (internal/llm)
        LLMProviderInterface(provider.go::LLMProvider Interface)
        LLMConcreteProviders(copilot_cli.go, gemini_cli.go, local_qwen.go)
        LLMProviderInterface <-- Implemented by --> LLMConcreteProviders
    end

    Workflow --- Uses defined LLM names --> LLMProviderInterface