# NS1 Service

A service to proxy requests to NS1 and store their result in a local datastore.

## Setup

Requirements: 
* Go (tested with 1.13)

Environment variables:
* `PORT` - port number of which to run the service, eg `8080`
* `NS1_API_KEY` - API key provided by NS1

To run the service:
```
go run .
```

## Usage

(I didn't have time to write a separate client for this service. This docs below show how to access this service using `curl`.)

### Create a new zone

PUT a JSON-represented zone to `/zones`. The JSON format for zones is provided by the [NS1 API docs](https://ns1.com/api). The `zone` field is required. All other fields are optional.

```
curl -X PUT -d '{ "zone":"netlify.com", "ttl":3600 }' http://localhost:8080/zones
```

Expected reponse:
```
{
  "dns_servers": [
    "dns1.p07.nsone.net",
    "dns2.p07.nsone.net",
    "dns3.p07.nsone.net",
    "dns4.p07.nsone.net"
  ],
  "message": "Set your domain's DNS servers to the hosts listed here. Normally you will do this in your domain registrar's portal. If this zone is a subdomain, you can do this by subdelegating the subdomain using NS records in the parent zone's DNS."
}
```

### Update a zone

POST a JSON-represented zone to `/zones/:zone`.

```
curl -X POST -d '{ "ttl":1337 }' http://localhost:8080/zones/netlify.com
```

Expected response: same as PUT.

### Delete a zone

Make a DELETE request to `/zones/:zone`.

```
curl -X DELETE http://localhost:8080/zones/netlify.com
```

### Add a record to a zone

PUT a JSON-represented record to `/zones/:zone/:domain`. The JSON format for records is provided by the [NS1 API docs](https://ns1.com/api).

```
curl -X PUT -d '{ "answers": [ { "answer": [ 10, "1.2.3.4" ] } ], "type": "MX", "domain": "foo.netlify.com", "zone": "netlify.com", "filters":[] }'
```

## Discussion of decisions

### JSON API

I decided to implement this service as a REST API mirror the one exposed by NS1. I chose a JSON API because it's easy to read the requests and responses and it's good communicating with most types of clients. Mirror the request formats expected by NS1's API was useful because their Go library already provides types for serializing and deserializing those requests.

For an internal-only service, a microservice that uses gRPC and protobuf to communicate could also be used.

### Datastore

I decided to use a key/value store for this project rather than a relational database like Postgres. I chose it because we don't need to join across records or query based on fields. It gave me more flexibility for storing the data.

For this assignment, I also liked the BoltDB is able to run in the same process as the webserver. That made configuration simpler and made it easy to create a temporary db for tests. I wanted to learn a new datastore (I've mostly used Postgres and MySQL) and this was my first time using BoltDB.

## Schema

I decided to store records as a field on zone documents. This seemed like a reasonable choise because it seemed unlikely that records would be so large that inserting a zone into them would be expensive. It seemed like a good opportunity to sync the zone with ns1 to make sure we have all the latest data.

One drawback of this approach is that there could be a bottleneck if many clients want to update records within the same zone.

Another potential issue is that we might recieve out of order responses from the NS1 service and attempt to apply a stale zone configuration to our datastore. The ability to update individual records would make this a bit less likely, since less data is overwrittent by a single update.

If the ns1 API returned a timestamp with API responses, we could throw out any attempted updated that have a timestamp that's earlier than the one in our datastore. Alternatively, we could have a per-zone lock on the entire operation of fetching a zone from NS1 and saving it to the datastore.

## Future work

* Complete routes for records. My intention that was that they would follow a similar pattern to `putRecord` - make a call to NS1's record service, then sync the zone to update the datastore.
* More tests
  * Add unit tests for db.go
  * Add tests for calls to the zones endpoints that don't result in success, including:
    * Request to NS1 errors
    * Request to NS1 returns non-200 response
    * An error occurs during out handling of their response
  * Add tests for records endpoints, similar to the tests for the zones endpoints.
* Respond consistently with JSON. Right now, some errors return a text body. It would be nice to return a JSONified error struct with a message field.
* Add some middleware
  * Log requests
  * Respond with `404` when a route is unrecognized
  * Handle panics by responding with `500`
* Write a client to consume this service
