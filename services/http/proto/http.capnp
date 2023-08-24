using Go = import "/go.capnp";

@0xbb59054ba43c3861;

$Go.package("pkg");
$Go.import("github.com/mikelsr/ww-webcrawler/services/http/proto/pkg");

interface Requester {
    get  @0 (url :Text) -> (response :Response);
    post @1 (url :Text, headers :List(Header), Body :Data) -> (response :Response);
    
    struct Header {
        key   @0 :Text;
        value @1 :Text;
    }

    struct Response {
        status @0 :UInt32;
        body   @1 :Data;
        error  @2 :Text;
    }
}