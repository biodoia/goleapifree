#!/bin/bash
# GoLeapAI Bash Completion Script
#
# To enable completion, add this to your .bashrc:
#   source /path/to/goleapai/scripts/completion.bash
#
# Or install system-wide:
#   sudo cp scripts/completion.bash /etc/bash_completion.d/goleapai

_goleapai_completion() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main commands
    commands="serve providers stats config migrate doctor version help"

    # Subcommands
    case "${COMP_WORDS[1]}" in
        providers)
            local provider_cmds="list add remove test sync"
            case "${prev}" in
                providers)
                    COMPREPLY=( $(compgen -W "${provider_cmds}" -- ${cur}) )
                    return 0
                    ;;
                --status)
                    COMPREPLY=( $(compgen -W "active deprecated down maintenance" -- ${cur}) )
                    return 0
                    ;;
                --type)
                    COMPREPLY=( $(compgen -W "free freemium paid local" -- ${cur}) )
                    return 0
                    ;;
                --auth-type)
                    COMPREPLY=( $(compgen -W "none api_key bearer oauth2" -- ${cur}) )
                    return 0
                    ;;
            esac
            ;;
        stats)
            local stats_cmds="show export reset"
            case "${prev}" in
                stats)
                    COMPREPLY=( $(compgen -W "${stats_cmds}" -- ${cur}) )
                    return 0
                    ;;
                --format)
                    COMPREPLY=( $(compgen -W "csv json" -- ${cur}) )
                    return 0
                    ;;
            esac
            ;;
        config)
            local config_cmds="show validate generate"
            case "${prev}" in
                config)
                    COMPREPLY=( $(compgen -W "${config_cmds}" -- ${cur}) )
                    return 0
                    ;;
                --env)
                    COMPREPLY=( $(compgen -W "development production" -- ${cur}) )
                    return 0
                    ;;
            esac
            ;;
        migrate)
            local migrate_cmds="up down reset seed status"
            case "${prev}" in
                migrate)
                    COMPREPLY=( $(compgen -W "${migrate_cmds}" -- ${cur}) )
                    return 0
                    ;;
            esac
            ;;
        doctor)
            case "${prev}" in
                --check)
                    COMPREPLY=( $(compgen -W "database redis providers" -- ${cur}) )
                    return 0
                    ;;
            esac
            ;;
    esac

    # Global flags
    case "${prev}" in
        --log-level|-l)
            COMPREPLY=( $(compgen -W "debug info warn error" -- ${cur}) )
            return 0
            ;;
        --config|-c)
            COMPREPLY=( $(compgen -f -X '!*.yaml' -- ${cur}) )
            return 0
            ;;
    esac

    # Complete with main commands if at root level
    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- ${cur}) )
        return 0
    fi
}

complete -F _goleapai_completion goleapai
