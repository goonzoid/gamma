# γ (gamma)

*like λ, but with more __#grime__*

![PARS R US](http://i.imgur.com/RrJjrFZ.gif)

## about

You can push nodejs programs into the cloud and then run them when events happen.

## usage

### installation

```
<change RECEPTOR environment variable in manifest.yml>

cf push
```

### create your package

γ should be able to run arbitrary nodejs packages, with one constraint: the entry point needs to be `bin/run`.

See `/example` in the repo for a simple example.

### register your function

Functions are registered by HTTP PUTing a nodejs package tarball (made with `npm pack`) to `/functions/:name`, passing the tarball as a form parameter named `tarball`.

See `/scripts/register_function` for an example.

### call your function

Functions are called by HTTP POSTing to `/functions/:name/call` with the environment variables you want to run your script with.

```
{
    "env": [
        {
            "name": "AWS_ACCESS_KEY_ID",
            "value": "$AWS_ACCESS_KEY_ID"
        },
        {
            "name": "AWS_SECRET_ACCESS_KEY",
            "value": "$AWS_SECRET_ACCESS_KEY"
        }
    ]
}

```

