# Planetarium

Planetarium is a small, simple server that hosts the CosmosHub Chain Registry. 

You can access it at [chain-registry.tessellated.io](https://chain-registry.tessellated.io)

## Installing

```
git clone https://github.com/tessellated-io/planetarium --recursive
make install
```

## API 

Files are exposed in the same way they are in the [Chain Registry](https://github.com/cosmos/chain-registry/). For instance, to find [`cosmoshub/chain.json`] you would simply `curl 127.0.0.1/cosmoshub.chain.json`.

## Updating

By default everything is static. You can refresh this periodically by running `make refresh`. If you have a use case to refresh automatically, please file a feature request. 