// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

#![allow(unused_imports)]
use std::error::Error;
use serde::{Serialize, Deserialize};
use atomic_counter::{AtomicCounter, RelaxedCounter};
use once_cell::sync::Lazy;
use lens_sdk::StreamOption;
use lens_sdk::option::StreamOption::{Some, None, EndOfStream};

// ETHABI_CLI
use ethabi_cli::decode;


#[link(wasm_import_module = "lens")]
extern "C" {
    fn next() -> *mut u8;
}

#[derive(Serialize, Deserialize)]
pub struct ResultValue {
    pub index_topic_0: String, // function signature
    pub index_topic_1: String, // from address
    pub index_topic_2: String, // to address
    pub value:         String, // token value
    pub index_topic_0: String // function signature
    pub index_topic_1: String // from address
    pub index_topic_2: String // to address
    pub value:         String // token value
}

#[no_mangle]
pub extern fn alloc(size: usize) -> *mut u8 {
    lens_sdk::alloc(size)
}

#[no_mangle]
pub extern fn transform() -> *mut u8 {
    match try_transform() {
        Ok(o) => match o {
            Some(result_json) => lens_sdk::to_mem(lens_sdk::JSON_TYPE_ID, &result_json),
            None => lens_sdk::nil_ptr(),
            EndOfStream => lens_sdk::to_mem(lens_sdk::EOS_TYPE_ID, &[]),
        },
        Err(e) => lens_sdk::to_mem(lens_sdk::ERROR_TYPE_ID, &e.to_string().as_bytes())
    }
}

fn try_transform() -> Result<StreamOption<Vec<u8>>, Box<dyn Error>> {
    let ptr = unsafe { next() };
    let input = match lens_sdk::try_from_mem::<Vec<String>>(ptr)? { // decode the input?
        Some(v) => v,
        // Implementations of `transform` are free to handle nil however they like. In this
        // implementation we chose to return nil given a nil input.
        None => return Ok(None),
        EndOfStream => return Ok(EndOfStream),
    };

    // use the rust library to generate the result

    let result = ResultValue {
        // use the rust library to generate the result
        index_topic_0: ethabi_cli::decode(input[0].as_bytes(), &ptr[1..]).unwrap(),
        index_topic_1: ethabi_cli::decode(input[1].as_bytes(), &ptr[1..]).unwrap(),
        index_topic_2: ethabi_cli::decode(input[2].as_bytes(), &ptr[1..]).unwrap(),
        value: ethabi_cli::decode(input[3].as_bytes(), &ptr[1..]).unwrap(),
        index_topic_0 = ethabi_cli.decode(input[0].as_bytes,&ptr[1..]).unwrap()
        index_topic_1 = ethabi_cli.decode(input[1].as_bytes,&ptr[1..]).unwrap()
        index_topic_2 = ethabi_cli.decode(input[2].as_bytes,&ptr[1..]).unwrap()
        value = ethabi_cli.decode(value.as_bytes,&ptr[1..]).unwrap()
    };

    let result_json = serde_json::to_vec(&result)?;
    lens_sdk::free_transport_buffer(ptr)?;
    Ok(Some(result_json))
}

