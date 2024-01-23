# Planetarium

Planetarium is a small, simple server that hosts the CosmosHub Chain Registry. 

You can access it at [chain-registry.tessellated.io](https://chain-registry.tessellated.io)

## Installing

```shell
$ git clone https://github.com/tessellated-io/planetarium # (Use --recursive if you want to pull chain-registry automatically).
$ make install
 
$ planetarium --help
```

## Usage

```shell
$ planetarium start --port 8080 --file-path /home/user/chain-registry
```

## API 

Files are exposed in the same way they are in the [Chain Registry](https://github.com/cosmos/chain-registry/). For instance, to find [`cosmoshub/chain.json`] you would simply `curl 127.0.0.1:5353/cosmoshub.chain.json`.

## Updates

You specify the directory that contains the chain registry, and thus can configure it to update however you want (manually, via a cron job, etc)
