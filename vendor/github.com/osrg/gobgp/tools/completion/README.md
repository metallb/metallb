# Completion

This page explains completion for gobgp client.

## Bash completion

The described how to use and how to customize of bash completion.

### How to use

1. install bash-completion as follows:

 ```
 % sudo apt-get install bash-completion
 ```

1. add gobgp's path to PATH environment variable

 If you run 'go get github.com/osrg/gobgp/gobgp', gobgp command is installed in $GOPATH/bin.
 ```
 % export PATH=$PATH:$GOPATH/bin
 ```

1. load completion file

 ```
 % source $GOPATH/src/github.com/osrg/gobgp/tools/completion/gobgp-completion.bash
 ```

You can use tab completion for gobgp after loading gobgp-completion.bash.

### How to customize
 In order to customize the bash completion, please follow steps below:

1. generate bash completion file

 Generate the bash completion file by using binary of gobgp client.
 This generating function uses [cobra bash completion](https://github.com/spf13/cobra#generating-bash-completions-for-your-command) internally.
 ```
 % gobgp --gen-cmpl --bash-cmpl-file=<specifying a file name>
 ```

 The following function is generated if added the "gobgp neighbor" command  to gobgp client.

 ```
 _gobgp_neighbor()
 {
     last_command="gobgp_neighbor"
     commands=()

     flags=()
     two_word_flags=()
     flags_with_completion=()
     flags_completion=()

     flags+=("--bash-cmpl-file=")
     flags+=("--debug")
     flags+=("-d")
     flags+=("--gen-cmpl")
     flags+=("-c")
     flags+=("--host=")
     two_word_flags+=("-u")
     flags+=("--json")
     flags+=("-j")
     flags+=("--port=")
     two_word_flags+=("-p")
     flags+=("--quiet")
     flags+=("-q")

     must_have_one_flag=()
     must_have_one_noun=()
 }
  ```

1. copy the generated functions

 Copy the above function to **gogbp-static-completion.bash**.

1. implement a dynamic completion

 If you want to add dynamic completion, you need to implement that part yourself.
 For example, if you want to add the neighbor address after the "gobgp neighbor" command dynamically, you can achieve it by implementing the specific internal command in **gobgp-dynamic-completion.bash**.

 1. implement command to get a list of the neighbor address

	You need to add the processing function of the following like below to **gobgp-dynamic-completion.bash**.
    ```
    # Get bgp neighbors by using gobgp command.
    __gobgp_q_neighbor()
    {
        local neighbors=( $(__gobgp_q $url $port --quiet neighbor) )
        case "${neighbors[*]}" in
            "grpc: timed out"* | "rpc error:"* )
                req_faild="True"
                return
            ;;
        esac
        for n in ${neighbors[*]}; do
            commands+=($n)
        done
        searched="True"
    }
    ```

 1. add a call to "__gobgp_q_neighbor"

	You can call the above functions by implementing as follows to **gobgp-static-completion.bash**:
    ```
    _gobgp_neighbor()
    {
        last_command="gobgp_neighbor"
        commands=()

        flags=()
        two_word_flags=()
        flags_with_completion=()
        flags_completion=()

        flags+=("--bash-cmpl-file=")
        flags+=("--debug")
        flags+=("-d")
        flags+=("--gen-cmpl")
        flags+=("-c")
        flags+=("--host=")
        two_word_flags+=("-u")
        flags+=("--json")
        flags+=("-j")
        flags+=("--port=")
        two_word_flags+=("-p")
        flags+=("--quiet")
        flags+=("-q")

        must_have_one_flag=()
        must_have_one_noun=()

        # Implement call processing to here
        __gobgp_q_neighbor
    }
    ```

 1. implement the handle processing

	If you want to add the completion following the "gobgp neighbor \<neighbor address\>" command, you need to add a handler for _gobgp_neighbor_addr() like below to "__handle_gobgp_command()" function in **gobgp-dynamic-completion.bash**.

    ```
    case "${last_command}" in
        # Control after dynamic completion of bgp neighbor command
        gobgp_neighbor )
            next_command="_${last_command}_addr"
        ;;
    esac
    ```

	"next_command" variable above indicates a function to be called after "gobgp neighbor", and the function name of next command is supposed to be "_gobgp_neighbor_addr" in the example above.
     Therefore the actual "gobgp_neighbor_addr" function needs to be implemented in **gobgp-dynamic-completion.bash**.
    ```
    _gobgp_neighbor_addr()
    {
        last_command="gobgp_neighbor_addr"

        commands=()
        commands+=("local")
        commands+=("adj-in")
        commands+=("adj-out")
        commands+=("reset")
        commands+=("softreset")
        commands+=("softresetin")
        commands+=("softresetout")
        commands+=("shutdown")
        commands+=("enable")
        commands+=("disable")
        commands+=("policy")

        flags=()
        two_word_flags=()
        flags_with_completion=()
        flags_completion=()

        flags+=("--address-family=")
        two_word_flags+=("-a")
        flags+=("--bash-cmpl-file=")
        flags+=("--debug")
        flags+=("-d")
        flags+=("--gen-cmpl")
        flags+=("-c")
        flags+=("--host=")
        two_word_flags+=("-u")
        flags+=("--json")
        flags+=("-j")
        flags+=("--port=")
        two_word_flags+=("-p")
        flags+=("--quiet")
        flags+=("-q")

        must_have_one_flag=()
        must_have_one_noun=()
    }
    ```

1. delete the generated bash completion file.

## Zsh completion

The described how to use of bash completion.

### How to use

zsh completion for gobgp works by adding the path of gobgp zsh completion directory to $fpath and enabling zsh completion like below:

 ```
 % vi ~/.zshrc

 GOBGP_COMP=$GOPATH/src/github.com/osrg/gobgp/tools/completion/zsh
 fpath=($GOBGP_COMP $fpath)

 autoload -Uz compinit
 compinit

 ```