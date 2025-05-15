use std::error::Error;
use serde::{Serialize, Deserialize};
use lens_sdk::StreamOption;
use lens_sdk::option::StreamOption::{Some, None, EndOfStream};

#[link(wasm_import_module = "lens")]
extern "C" {
    fn next() -> *mut u8;
}

#[derive(Serialize)]
pub struct Output {
    pub index_topic_0: String,
    pub index_topic_1: String,
    pub index_topic_2: String,
    pub index_topic_3: String,
    pub index_topic_4: String,
    pub value:         String,
}

#[derive(Deserialize)]
pub struct Input {
    pub topics: String,
    pub number: u64,
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
        Err(e) => lens_sdk::to_mem(lens_sdk::ERROR_TYPE_ID, &e.to_string().as_bytes()),
    }
}

fn try_transform() -> Result<StreamOption<Vec<u8>>, Box<dyn Error>> {
    let ptr = unsafe { next() };
    let input = match lens_sdk::try_from_mem::<Input>(ptr)? {
        Some(v) => v,
        None => return Ok(None),
        EndOfStream => return Ok(EndOfStream),
    };

    let topics: Vec<&str> = input.topics.split(';').collect();
    let mut result = Output {
        index_topic_0: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_1: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_2: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_3: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        index_topic_4: "0x0000000000000000000000000000000000000000000000000000000000000000".to_string(),
        value: "".to_string(),
    };

    for i in 0..input.number {
        let idx = i as usize;
        if idx < topics.len() {
            match i {
                0 => result.index_topic_0 = topics[idx].to_string(), // TODO: decode
                1 => result.index_topic_1 = topics[idx].to_string(), // TODO: decode
                2 => result.index_topic_2 = topics[idx].to_string(), // TODO: decode
                3 => result.index_topic_3 = topics[idx].to_string(), // TODO: decode
                4 => result.index_topic_4 = topics[idx].to_string(), // TODO: decode
                _ => break,
            }
        } else {
            break;
        }
    }

    let result_json = serde_json::to_vec(&result)?;
    lens_sdk::free_transport_buffer(ptr)?;
    Ok(Some(result_json))
}

